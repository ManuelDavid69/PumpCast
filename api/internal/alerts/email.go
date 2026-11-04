package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"
)

// Message est un email prêt à partir.
type Message struct {
	To      string
	Subject string
	Text    string
}

// Sender envoie un email. Deux versions : une qui écrit juste dans les logs
// (en local) et une qui passe par Brevo (en production).
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// LogSender affiche l'email au lieu de l'envoyer, pratique en développement.
type LogSender struct{}

func (LogSender) Send(_ context.Context, m Message) error {
	log.Printf("email (non envoyé, EMAIL_PROVIDER=log) à %s : %s\n%s", m.To, m.Subject, m.Text)
	return nil
}

// BrevoSender envoie par l'API de Brevo (offre gratuite : 300 emails par jour,
// et pas besoin d'avoir un nom de domaine, une adresse vérifiée suffit).
type BrevoSender struct {
	APIKey    string
	FromEmail string
	FromName  string
	URL       string // remplacée dans les tests
	Client    *http.Client
}

const brevoURL = "https://api.brevo.com/v3/smtp/email"

func (b BrevoSender) Send(ctx context.Context, m Message) error {
	type contact struct {
		Email string `json:"email"`
		Name  string `json:"name,omitempty"`
	}
	body, err := json.Marshal(map[string]any{
		"sender":      contact{Email: b.FromEmail, Name: b.FromName},
		"to":          []contact{{Email: m.To}},
		"subject":     m.Subject,
		"textContent": m.Text,
		"htmlContent": textToHTML(m.Text),
	})
	if err != nil {
		return err
	}
	url := b.URL
	if url == "" {
		url = brevoURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("api-key", b.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := b.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("envoi Brevo : %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return fmt.Errorf("envoi Brevo : statut %d : %s", resp.StatusCode, detail)
	}
	return nil
}

// textToHTML met le texte dans du HTML très simple. html.EscapeString évite
// qu'une adresse de station bizarre casse le HTML de l'email.
func textToHTML(text string) string {
	var b strings.Builder
	b.WriteString(`<div style="font-family: sans-serif; font-size: 15px; line-height: 1.5; color: #1b2430">`)
	for _, para := range strings.Split(text, "\n\n") {
		b.WriteString("<p>")
		b.WriteString(strings.ReplaceAll(html.EscapeString(para), "\n", "<br>"))
		b.WriteString("</p>")
	}
	b.WriteString("</div>")
	return b.String()
}

var fuelLabels = map[string]string{
	"GAZOLE": "Gazole", "SP95": "SP95", "SP98": "SP98", "E10": "SP95-E10", "E85": "E85", "GPLC": "GPLc",
}

var weekdays = [...]string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"}

func euros(v float64) string {
	return strings.Replace(fmt.Sprintf("%.3f €", v), ".", ",", 1)
}

// StationInfo est ce qu'il faut pour écrire l'email.
type StationInfo struct {
	ID      string
	Address string
	City    string
}

// BuildMessage écrit l'email d'une alerte déclenchée.
func BuildMessage(to string, a Alert, st StationInfo, s Situation, siteURL string) Message {
	fuel := fuelLabels[a.Fuel]
	city := tidyName(st.City)
	where := fmt.Sprintf("%s, %s", tidyName(st.Address), city)
	link := fmt.Sprintf("%s/stations/%s?carburant=%s", strings.TrimRight(siteURL, "/"), st.ID, a.Fuel)
	footer := fmt.Sprintf("Tu reçois cet email parce que tu as créé une alerte sur PumpCast. "+
		"Pour la supprimer : %s/compte", strings.TrimRight(siteURL, "/"))

	var subject, body string
	switch a.Type {
	case TypeThreshold:
		subject = fmt.Sprintf("%s à %s à %s", fuel, euros(s.Price), city)
		body = fmt.Sprintf("Le %s est passé à %s à la station %s (ton seuil : %s).",
			fuel, euros(s.Price), where, euros(*a.Threshold))
	case TypePredictedDrop:
		subject = fmt.Sprintf("Baisse probable du %s à %s", fuel, city)
		body = fmt.Sprintf("D'après la prévision du jour, le %s devrait descendre vers %s d'ici %s à la station %s. "+
			"Ça vaut peut-être le coup d'attendre avant de faire le plein.\n\n"+
			"La prévision peut se tromper, ses résultats sont détaillés ici : %s/modele",
			fuel, euros(s.TargetPrice), weekdays[s.TargetDate.Weekday()], where, strings.TrimRight(siteURL, "/"))
	case TypePredictedRise:
		subject = fmt.Sprintf("Le %s devrait monter à %s", fuel, city)
		body = fmt.Sprintf("D'après la prévision du jour, le %s devrait monter vers %s d'ici %s à la station %s "+
			"(il est à %s aujourd'hui). Si tu dois faire le plein, c'est plutôt le moment.",
			fuel, euros(s.TargetPrice), weekdays[s.TargetDate.Weekday()], where, euros(s.Price))
	}
	return Message{
		To:      to,
		Subject: subject,
		Text:    fmt.Sprintf("Bonjour,\n\n%s\n\nVoir la station : %s\n\n%s", body, link, footer),
	}
}

// tidyName remet en casse normale les noms écrits tout en majuscules dans le
// flux officiel ("VILLEURBANNE" devient "Villeurbanne"). Un nom déjà en
// minuscules est laissé tel quel. Même règle que formatName côté front.
func tidyName(s string) string {
	if s != strings.ToUpper(s) {
		return s
	}
	runes := []rune(strings.ToLower(s))
	upper := true
	for i, r := range runes {
		if upper && unicode.IsLetter(r) {
			runes[i] = unicode.ToUpper(r)
		}
		upper = r == ' ' || r == '-' || r == '\''
	}
	return string(runes)
}
