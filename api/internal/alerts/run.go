package alerts

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/ManuelDavid69/pumpcast/api/internal/predictions"
	"github.com/ManuelDavid69/pumpcast/api/internal/prices"
	"github.com/ManuelDavid69/pumpcast/api/internal/stations"
	"github.com/ManuelDavid69/pumpcast/api/internal/users"
)

// Checker vérifie toutes les alertes actives et envoie les emails.
// Il tourne après chaque calcul des prévisions (une fois par jour) et après
// les ingestions pour les alertes de seuil.
type Checker struct {
	Alerts      *Repository
	Users       *users.Repository
	Stations    *stations.Repository
	Current     *prices.CurrentCache
	Predictions *predictions.Repository
	Sender      Sender
	SiteURL     string
	Now         func() time.Time
}

type Summary struct {
	Checked int
	Sent    int
	Failed  int
}

func (s Summary) String() string {
	return fmt.Sprintf("%d alertes vérifiées, %d emails envoyés, %d échecs", s.Checked, s.Sent, s.Failed)
}

func (c *Checker) Run(ctx context.Context) (Summary, error) {
	var sum Summary
	list, err := c.Alerts.Active(ctx)
	if err != nil || len(list) == 0 {
		return sum, err
	}

	// Toutes les données en quelques requêtes groupées, pas une par alerte
	stationIDs := map[string]bool{}
	byFuel := map[string][]string{}
	userIDs := map[bson.ObjectID]bool{}
	for _, a := range list {
		stationIDs[a.StationID] = true
		byFuel[a.Fuel] = append(byFuel[a.Fuel], a.StationID)
		userIDs[a.UserID] = true
	}
	ids := keys(stationIDs)
	current, err := c.Current.GetMany(ctx, ids)
	if err != nil {
		return sum, err
	}
	docs, err := c.Stations.ByIDs(ctx, ids)
	if err != nil {
		return sum, err
	}
	preds := map[string]map[string]*predictions.Prediction{}
	for fuel, sids := range byFuel {
		if preds[fuel], err = c.Predictions.LatestForStations(ctx, sids, fuel); err != nil {
			return sum, err
		}
	}
	accounts, err := c.Users.ByIDs(ctx, keys(userIDs))
	if err != nil {
		return sum, err
	}

	now := c.Now()
	for _, a := range list {
		sum.Checked++
		s := situation(current[a.StationID], preds[a.Fuel][a.StationID], a.Fuel)
		send, triggered := Decide(a, ConditionMet(a, s), now)

		var sentAt *time.Time
		if send {
			user, okUser := accounts[a.UserID]
			doc, okStation := docs[a.StationID]
			if okUser && okStation {
				msg := BuildMessage(user.Email, a, StationInfo{ID: doc.ID, Address: doc.Address, City: doc.City}, s, c.SiteURL)
				if err := c.Sender.Send(ctx, msg); err != nil {
					// Pas marquée comme envoyée : on réessaiera à la prochaine vérification
					log.Printf("alerte %s : envoi échoué : %v", a.ID.Hex(), err)
					sum.Failed++
					triggered = false
				} else {
					sum.Sent++
					sentAt = &now
				}
			}
		}
		if triggered != a.Triggered || sentAt != nil {
			if err := c.Alerts.SetState(ctx, a.ID, triggered, sentAt); err != nil {
				return sum, err
			}
		}
	}
	return sum, nil
}

func situation(cur []prices.Current, p *predictions.Prediction, fuel string) Situation {
	var s Situation
	for _, c := range cur {
		if c.Fuel == fuel && !c.OutOfStock && c.Price > 0 {
			s.Price, s.HasPrice = c.Price, true
		}
	}
	if p != nil && p.Recommendation != nil {
		s.Recommendation = *p.Recommendation
		if p.AdviceTarget != nil {
			s.TargetPrice, s.TargetDate = p.AdviceTarget.Price, p.AdviceTarget.Date
		}
	}
	return s
}

func keys[K comparable](m map[K]bool) []K {
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
