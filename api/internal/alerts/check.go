package alerts

import (
	"time"
)

// Délai minimum entre deux emails pour la même alerte, au cas où le prix
// passe et repasse le seuil plusieurs fois dans la journée
const Cooldown = 12 * time.Hour

// Situation est ce qu'on sait d'une station pour un carburant au moment de
// la vérification.
type Situation struct {
	Price    float64 // prix actuel, 0 si inconnu ou en rupture
	HasPrice bool
	// Conseil du jour du modèle : "wait", "fill_now", "stable" ou "" (aucun)
	Recommendation string
	TargetPrice    float64 // prix visé par le conseil
	TargetDate     time.Time
}

// ConditionMet dit si l'alerte doit se déclencher dans cette situation.
func ConditionMet(a Alert, s Situation) bool {
	switch a.Type {
	case TypeThreshold:
		return s.HasPrice && a.Threshold != nil && s.Price <= *a.Threshold
	case TypePredictedDrop:
		return s.Recommendation == "wait"
	case TypePredictedRise:
		return s.Recommendation == "fill_now"
	}
	return false
}

// Decide dit s'il faut envoyer un email et quel est le nouvel état de l'alerte.
//
// On n'envoie qu'au moment où la condition devient vraie (Triggered passe de
// false à true). Tant qu'elle reste vraie, pas de nouvel email. Quand elle
// redevient fausse, l'alerte est réarmée pour la prochaine fois.
func Decide(a Alert, conditionMet bool, now time.Time) (send bool, triggered bool) {
	if !conditionMet {
		return false, false
	}
	if a.Triggered {
		return false, true
	}
	if a.LastSentAt != nil && now.Sub(*a.LastSentAt) < Cooldown {
		return false, true
	}
	return true, true
}
