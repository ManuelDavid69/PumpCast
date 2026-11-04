// Package model contient les types exposés par l'API GraphQL.
//
// Station et DepartmentStats sont écrits à la main : leurs champs lourds
// (prix actuels, historique, prédiction, station la moins chère...) ne sont
// pas des champs de la struct mais des resolvers appelés seulement si le
// client les demande.
package model

import "github.com/ManuelDavid69/pumpcast/api/internal/stations"

type Station struct {
	ID           string
	Address      string
	City         string
	PostalCode   string
	Department   string
	Latitude     *float64
	Longitude    *float64
	RoadType     string
	Services     []string
	OpeningHours *OpeningHours
	DistanceKm   *float64
}

type DepartmentStats struct {
	Department   string
	Fuel         Fuel
	Date         string
	AvgPrice     float64
	MinPrice     float64
	MaxPrice     float64
	StationCount int
}

// StationFromDoc convertit un document MongoDB en type GraphQL.
func StationFromDoc(doc stations.Station) *Station {
	s := &Station{
		ID:         doc.ID,
		Address:    doc.Address,
		City:       doc.City,
		PostalCode: doc.PostalCode,
		Department: doc.Department,
		RoadType:   doc.RoadType,
		Services:   doc.Services,
	}
	if s.Services == nil {
		s.Services = []string{}
	}
	if doc.Location != nil && len(doc.Location.Coordinates) == 2 {
		lng, lat := doc.Location.Coordinates[0], doc.Location.Coordinates[1]
		s.Latitude, s.Longitude = &lat, &lng
	}

	hours := &OpeningHours{Automate24h: doc.OpeningHours.Automate24h, Days: []*OpeningDay{}}
	for _, d := range doc.OpeningHours.Days {
		day := &OpeningDay{Day: d.Day, Name: d.Name, Closed: d.Closed, Slots: []*TimeSlot{}}
		for _, sl := range d.Slots {
			day.Slots = append(day.Slots, &TimeSlot{Open: sl.Open, Close: sl.Close})
		}
		hours.Days = append(hours.Days, day)
	}
	s.OpeningHours = hours
	return s
}
