package resolver

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var repositories = []string{
	"https://dl.google.com/dl/android/maven2",
	"https://repo.maven.apache.org/maven2",
	"https://repo1.maven.org/maven2",
}

var artifactHTTPClient = &http.Client{Timeout: 45 * time.Second}

// fetchArtifact intenta descargar un artefacto desde los repositorios configurados.
// Los 404 pasan al siguiente repositorio; los 429 y errores temporales se reintentan.
func fetchArtifact(url string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "Rain-Android-Builder/1.0")

		resp, err := artifactHTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("error de red: %w", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			status := resp.StatusCode
			retryAfter := resp.Header.Get("Retry-After")
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d en %s", status, url)

			if status == http.StatusTooManyRequests || status >= 500 {
				delay := 700 * time.Millisecond
				if retryAfter != "" {
					if seconds, parseErr := time.ParseDuration(retryAfter + "s"); parseErr == nil {
						delay = seconds
					}
				}
				time.Sleep(delay)
				continue
			}
			return "", lastErr
		}

		tmp, err := os.CreateTemp("", "rain-artifact-*")
		if err != nil {
			resp.Body.Close()
			return "", err
		}
		_, copyErr := io.Copy(tmp, resp.Body)
		resp.Body.Close()
		closeErr := tmp.Close()
		if copyErr != nil {
			os.Remove(tmp.Name())
			return "", copyErr
		}
		if closeErr != nil {
			os.Remove(tmp.Name())
			return "", closeErr
		}
		return tmp.Name(), nil
	}
	return "", lastErr
}

// DownloadAAR descarga un AAR de los repositorios y extrae classes.jar.
func DownloadAAR(coordinate string, destDir string) (string, error) {
	parts := strings.Split(coordinate, ":")
	if len(parts) != 3 {
		return "", fmt.Errorf("coordenada inválida: %s", coordinate)
	}
	group, artifact, version := parts[0], parts[1], parts[2]
	groupPath := strings.ReplaceAll(group, ".", "/")

	var lastErr error
	for _, repo := range repositories {
		url := fmt.Sprintf("%s/%s/%s/%s/%s-%s.aar", repo, groupPath, artifact, version, artifact, version)
		tmpAAR, err := fetchArtifact(url)
		if err != nil {
			lastErr = err
			continue
		}
		if err := os.MkdirAll(destDir, 0755); err != nil {
			os.Remove(tmpAAR)
			return "", err
		}

		zr, err := zip.OpenReader(tmpAAR)
		if err != nil {
			os.Remove(tmpAAR)
			lastErr = fmt.Errorf("AAR no es ZIP válido: %w", err)
			continue
		}

		classesJarPath := ""
		for _, zf := range zr.File {
			if zf.Name != "classes.jar" {
				continue
			}
			classesJarPath = filepath.Join(destDir, artifact+"-"+version+"-classes.jar")
			out, createErr := os.Create(classesJarPath)
			if createErr != nil {
				zr.Close()
				os.Remove(tmpAAR)
				return "", createErr
			}
			rc, openErr := zf.Open()
			if openErr != nil {
				out.Close()
				zr.Close()
				os.Remove(tmpAAR)
				return "", openErr
			}
			_, copyErr := io.Copy(out, rc)
			rc.Close()
			out.Close()
			if copyErr != nil {
				zr.Close()
				os.Remove(tmpAAR)
				return "", copyErr
			}
			break
		}
		zr.Close()
		os.Remove(tmpAAR)

		if classesJarPath != "" {
			return classesJarPath, nil
		}
		lastErr = fmt.Errorf("classes.jar no encontrado en AAR")
	}
	return "", fmt.Errorf("no se encontró AAR para %s (último error: %v)", coordinate, lastErr)
}

// DownloadArtifact descarga un artefacto y devuelve la ruta para el classpath.
// Prueba JAR antes que AAR porque la mayoría de las dependencias Maven son JAR.
// Si no existe como JAR, intenta AAR y extrae classes.jar.
func DownloadArtifact(coordinate string, destDir string) (string, error) {
	parts := strings.Split(coordinate, ":")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", fmt.Errorf("coordenada inválida: %s", coordinate)
	}
	group, artifact, version := parts[0], parts[1], parts[2]
	groupPath := strings.ReplaceAll(group, ".", "/")

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	var errorsFound []string
	for _, ext := range []string{"jar", "aar"} {
		for _, repo := range repositories {
			url := fmt.Sprintf("%s/%s/%s/%s/%s-%s.%s", repo, groupPath, artifact, version, artifact, version, ext)
			tmpPath, err := fetchArtifact(url)
			if err != nil {
				errorsFound = append(errorsFound, err.Error())
				continue
			}

			destPath := filepath.Join(destDir, fmt.Sprintf("%s-%s.%s", artifact, version, ext))
			if err := os.Rename(tmpPath, destPath); err != nil {
				os.Remove(tmpPath)
				return "", err
			}

			if ext == "jar" {
				return destPath, nil
			}

			zr, err := zip.OpenReader(destPath)
			if err != nil {
				os.Remove(destPath)
				errorsFound = append(errorsFound, fmt.Sprintf("AAR inválido: %v", err))
				continue
			}

			classesJarPath := filepath.Join(destDir, artifact+"-"+version+"-classes.jar")
			found := false
			for _, zf := range zr.File {
				if zf.Name != "classes.jar" {
					continue
				}
				out, createErr := os.Create(classesJarPath)
				if createErr != nil {
					zr.Close()
					return "", createErr
				}
				rc, openErr := zf.Open()
				if openErr != nil {
					out.Close()
					zr.Close()
					return "", openErr
				}
				_, copyErr := io.Copy(out, rc)
				rc.Close()
				out.Close()
				if copyErr != nil {
					zr.Close()
					return "", copyErr
				}
				found = true
				break
			}
			zr.Close()
			os.Remove(destPath)
			if found {
				return classesJarPath, nil
			}
			errorsFound = append(errorsFound, "classes.jar no encontrado en AAR")
		}
	}

	return "", fmt.Errorf("no se encontró JAR ni AAR para %s: %s", coordinate, strings.Join(errorsFound, "; "))
}
