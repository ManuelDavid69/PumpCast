// Package ingest télécharge et lit le flux officiel des prix des carburants.
//
// Le flux instantané et les archives annuelles ont le même format XML
// (<pdv_liste> avec un <pdv> par station), donc le même parser sert aux deux.
package ingest

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // embarque les fuseaux horaires, l'image distroless n'en a pas forcément

	"golang.org/x/text/encoding/charmap"

	"github.com/ManuelDavid69/pumpcast/api/internal/stations"
)

// Paris est le fuseau des dates du flux, qui sont données sans fuseau.
var Paris = mustLoadLocation("Europe/Paris")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// Station est une station déjà nettoyée, prête à être enregistrée.
type Station struct {
	ID         string
	Latitude   float64
	Longitude  float64
	HasCoords  bool
	PostalCode string
	Department string
	Address    string
	City       string
	RoadType   string // "R" route, "A" autoroute
	Services   []string
	Hours      stations.OpeningHours
	Prices     []Price
	Ruptures   []Rupture
}

// Price est le prix d'un carburant à une date de mise à jour.
type Price struct {
	Fuel      string // GAZOLE, SP95, SP98, E10, E85, GPLC
	Value     float64
	UpdatedAt time.Time
}

// Rupture est une rupture de stock signalée par la station.
type Rupture struct {
	Fuel  string
	Start time.Time
	End   time.Time // zéro si la rupture est toujours en cours
	Type  string    // "temporaire" ou "definitive"
}

// Ongoing dit si la rupture n'est pas terminée.
func (r Rupture) Ongoing() bool { return r.End.IsZero() }

// Structures qui collent au XML. On les convertit ensuite en Station.
type xmlPDV struct {
	ID        string `xml:"id,attr"`
	Latitude  string `xml:"latitude,attr"`
	Longitude string `xml:"longitude,attr"`
	CP        string `xml:"cp,attr"`
	Pop       string `xml:"pop,attr"`
	Adresse   string `xml:"adresse"`
	Ville     string `xml:"ville"`
	Horaires  *struct {
		Automate string `xml:"automate-24-24,attr"`
		Jours    []struct {
			ID       int    `xml:"id,attr"`
			Nom      string `xml:"nom,attr"`
			Ferme    string `xml:"ferme,attr"`
			Horaires []struct {
				Ouverture string `xml:"ouverture,attr"`
				Fermeture string `xml:"fermeture,attr"`
			} `xml:"horaire"`
		} `xml:"jour"`
	} `xml:"horaires"`
	Services []string `xml:"services>service"`
	Prix     []struct {
		Nom    string `xml:"nom,attr"`
		Maj    string `xml:"maj,attr"`
		Valeur string `xml:"valeur,attr"`
	} `xml:"prix"`
	Ruptures []struct {
		Nom   string `xml:"nom,attr"`
		Debut string `xml:"debut,attr"`
		Fin   string `xml:"fin,attr"`
		Type  string `xml:"type,attr"`
	} `xml:"rupture"`
}

// Parse lit le XML station par station et appelle fn pour chacune.
// On ne charge jamais tout le fichier en mémoire : les archives annuelles
// font plusieurs centaines de Mo.
// Une station mal formée est ignorée (on compte juste les rejets) pour
// qu'une ligne bizarre ne bloque pas toute l'ingestion.
func Parse(r io.Reader, fn func(Station) error) (skipped int, err error) {
	dec := xml.NewDecoder(r)
	// Le flux est en ISO-8859-1, encoding/xml ne gère que l'UTF-8 tout seul
	dec.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		switch strings.ToLower(label) {
		case "iso-8859-1", "latin1":
			return charmap.ISO8859_1.NewDecoder().Reader(input), nil
		case "windows-1252":
			return charmap.Windows1252.NewDecoder().Reader(input), nil
		}
		return nil, fmt.Errorf("encodage non géré : %s", label)
	}

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return skipped, nil
		}
		if err != nil {
			return skipped, fmt.Errorf("lecture xml : %w", err)
		}

		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "pdv" {
			continue
		}

		var raw xmlPDV
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return skipped, fmt.Errorf("décodage d'un pdv : %w", err)
		}

		st, ok := convert(raw)
		if !ok {
			skipped++
			continue
		}
		if err := fn(st); err != nil {
			return skipped, err
		}
	}
}

func convert(raw xmlPDV) (Station, bool) {
	id := strings.TrimSpace(raw.ID)
	cp := strings.TrimSpace(raw.CP)
	if id == "" || cp == "" {
		return Station{}, false
	}

	st := Station{
		ID:         id,
		PostalCode: cp,
		Department: DepartmentFromPostalCode(cp),
		Address:    strings.TrimSpace(raw.Adresse),
		City:       strings.TrimSpace(raw.Ville),
		RoadType:   strings.ToUpper(strings.TrimSpace(raw.Pop)),
		Services:   raw.Services,
	}
	if st.Department == "" {
		return Station{}, false
	}
	if st.Services == nil {
		st.Services = []string{}
	}

	lat, errLat := parseCoord(raw.Latitude, 90)
	lng, errLng := parseCoord(raw.Longitude, 180)
	if errLat == nil && errLng == nil && (lat != 0 || lng != 0) {
		st.Latitude, st.Longitude, st.HasCoords = lat, lng, true
	}

	if raw.Horaires != nil {
		st.Hours.Automate24h = raw.Horaires.Automate == "1"
		for _, j := range raw.Horaires.Jours {
			day := stations.Day{Day: j.ID, Name: j.Nom, Closed: j.Ferme == "1"}
			if !day.Closed {
				for _, h := range j.Horaires {
					day.Slots = append(day.Slots, stations.Slot{Open: cleanHour(h.Ouverture), Close: cleanHour(h.Fermeture)})
				}
			}
			st.Hours.Days = append(st.Hours.Days, day)
		}
	}

	for _, p := range raw.Prix {
		fuel := NormalizeFuel(p.Nom)
		if fuel == "" {
			continue
		}
		value, err := parsePrice(p.Valeur)
		if err != nil {
			continue
		}
		maj, err := parseDate(p.Maj)
		if err != nil {
			continue
		}
		st.Prices = append(st.Prices, Price{Fuel: fuel, Value: value, UpdatedAt: maj})
	}

	for _, r := range raw.Ruptures {
		fuel := NormalizeFuel(r.Nom)
		if fuel == "" {
			continue
		}
		start, err := parseDate(r.Debut)
		if err != nil {
			continue
		}
		rup := Rupture{Fuel: fuel, Start: start, Type: strings.ToLower(strings.TrimSpace(r.Type))}
		if end, err := parseDate(r.Fin); err == nil {
			rup.End = end
		}
		st.Ruptures = append(st.Ruptures, rup)
	}

	return st, true
}

// NormalizeFuel convertit le nom du flux en valeur de l'enum GraphQL.
// Renvoie "" pour un carburant qu'on ne suit pas.
func NormalizeFuel(name string) string {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "GAZOLE":
		return "GAZOLE"
	case "SP95":
		return "SP95"
	case "SP98":
		return "SP98"
	case "E10", "SP95-E10":
		return "E10"
	case "E85":
		return "E85"
	case "GPLC", "GPL":
		return "GPLC"
	}
	return ""
}

// DepartmentFromPostalCode déduit le département du code postal.
// Cas particuliers : la Corse (2A / 2B) et l'outre-mer (3 chiffres).
func DepartmentFromPostalCode(cp string) string {
	if len(cp) != 5 {
		return ""
	}
	n, err := strconv.Atoi(cp)
	if err != nil {
		return ""
	}
	switch {
	case strings.HasPrefix(cp, "97"), strings.HasPrefix(cp, "98"):
		return cp[:3]
	case strings.HasPrefix(cp, "20"):
		// 200xx et 201xx : Corse-du-Sud, le reste : Haute-Corse
		if n < 20200 {
			return "2A"
		}
		return "2B"
	}
	return cp[:2]
}

// parseCoord lit une coordonnée. Le flux les donne multipliées par 100 000
// ("4556000" pour 45.56), mais on accepte aussi des degrés directs.
func parseCoord(s string, limit float64) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("coordonnée vide")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if math.Abs(v) > limit {
		v /= 100000
	}
	if math.Abs(v) > limit {
		return 0, fmt.Errorf("coordonnée hors limites : %s", s)
	}
	return v, nil
}

// parsePrice lit un prix en euros. Les vieilles archives donnent les prix
// en millièmes d'euro ("1459" pour 1,459 €).
func parsePrice(s string) (float64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, err
	}
	if v > 100 {
		v /= 1000
	}
	// Hors de 0,30 € - 5 € c'est une faute de frappe de la station (vu dans les
	// archives : un gazole à 8,82 €, de l'E85 à 0,13 €). Les valeurs moins
	// grossières seront filtrées plus tard par rapport au prix moyen du département.
	if v < 0.3 || v > 5 {
		return 0, fmt.Errorf("prix incohérent : %s", s)
	}
	return v, nil
}

// parseDate lit les deux formats rencontrés : "2026-11-03 00:01:00"
// (flux actuel) et "2007-01-02T09:01:00" (anciennes archives).
func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("date vide")
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, s, Paris); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("date illisible : %s", s)
}

// cleanHour passe de "07.00" à "07:00".
func cleanHour(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), ".", ":")
}
