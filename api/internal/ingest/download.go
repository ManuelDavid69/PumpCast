package ingest

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Le zip du flux instantané fait environ 1 Mo, on refuse tout ce qui
// dépasse largement pour ne pas remplir la mémoire si l'URL renvoie n'importe quoi.
const maxDownloadSize = 200 << 20

// Download récupère le zip du flux et renvoie le contenu du XML qu'il contient.
func Download(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "PumpCast-ingest/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("téléchargement du flux : %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("téléchargement du flux : statut HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadSize+1))
	if err != nil {
		return nil, fmt.Errorf("lecture du flux : %w", err)
	}
	if len(body) > maxDownloadSize {
		return nil, fmt.Errorf("flux trop gros (> %d octets)", maxDownloadSize)
	}
	return ExtractXML(body)
}

// ReadFile lit un fichier local, zip ou XML. Pratique pour tester sans réseau.
func ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(path), ".xml") {
		return data, nil
	}
	return ExtractXML(data)
}

// ExtractXML renvoie le premier fichier .xml trouvé dans le zip.
func ExtractXML(zipData []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("ouverture du zip : %w", err)
	}
	for _, f := range zr.File {
		if !strings.EqualFold(filepath.Ext(f.Name), ".xml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxDownloadSize*4))
		closeErr := rc.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return data, nil
	}
	return nil, fmt.Errorf("aucun fichier xml dans le zip")
}
