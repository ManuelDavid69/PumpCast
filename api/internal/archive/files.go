package archive

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// URL des archives annuelles, l'année est ajoutée à la fin
const BaseURL = "https://donnees.roulez-eco.fr/opendata/annee/"

// Fetch télécharge le zip d'une année dans dir, sauf s'il est déjà là
// (et téléchargé aujourd'hui pour l'année en cours, qui change tous les jours).
// Le fichier est écrit directement sur le disque, sans passer par la mémoire.
func Fetch(ctx context.Context, dir string, year int, currentYear bool) (string, error) {
	path := filepath.Join(dir, fmt.Sprintf("%d.zip", year))
	if info, err := os.Stat(path); err == nil {
		fresh := time.Since(info.ModTime()) < 24*time.Hour
		if !currentYear || fresh {
			return path, nil
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s%d", BaseURL, year), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "PumpCast-import/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("téléchargement de l'archive %d : %w", year, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("téléchargement de l'archive %d : statut HTTP %d", year, resp.StatusCode)
	}

	// On écrit dans un fichier temporaire puis on renomme : un téléchargement
	// coupé en plein milieu ne laisse pas un zip abîmé en cache
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", fmt.Errorf("téléchargement de l'archive %d : %w", year, err)
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return path, nil
}

// OpenXML ouvre le XML contenu dans le zip, en lecture au fil de l'eau.
// Il faut appeler la fonction close renvoyée une fois la lecture finie.
func OpenXML(zipPath string) (io.Reader, func() error, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, fmt.Errorf("ouverture de %s : %w", zipPath, err)
	}
	for _, f := range zr.File {
		if !strings.EqualFold(filepath.Ext(f.Name), ".xml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			_ = zr.Close()
			return nil, nil, err
		}
		closeAll := func() error {
			errFile := rc.Close()
			if err := zr.Close(); err != nil {
				return err
			}
			return errFile
		}
		return rc, closeAll, nil
	}
	_ = zr.Close()
	return nil, nil, fmt.Errorf("aucun xml dans %s", zipPath)
}
