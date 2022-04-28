package archiveorg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

const (
	archiveApi      string = "https://web.archive.org"
	canonicalSearch string = "<link rel=\"canonical\" href=\""
)

type ArchiveOrgWaybackResponse struct {
	URL               string `json:"url"`
	ArchivedSnapshots struct {
		Closest struct {
			Status    string `json:"status"`
			Available bool   `json:"available"`
			URL       string `json:"url"`
			Timestamp string `json:"timestamp"`
		} `json:"closest"`
	} `json:"archived_snapshots"`
}

// Gets the most recent archive.org URL for a url and a boolean whether or not to
// archive the page if not found. Returns the latest archive.org URL for the page
// and a boolean whether or not the page existed
func GetLatestURL(url string) (archiveUrl string, exists bool, err error) {
	http := http.Client{}
	resp, err := http.Get(archiveApi + "/wayback/available?url=" + url)
	if err != nil {
		return "", false, fmt.Errorf("error calling wayback api: %w", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("error reading body from wayback api: %w", err)
	}

	var r ArchiveOrgWaybackResponse
	err = json.Unmarshal(body, &r)
	if err != nil {
		return "", false, fmt.Errorf("error unmarshalling json: %w, body: %v", err, string(body))
	}

	if r.ArchivedSnapshots.Closest.URL == "" {
		return "", false, nil
	}

	return r.ArchivedSnapshots.Closest.URL, true, nil
}

// Takes a slice of strings and a boolean whether or not to archive the page if not found
// and returns a slice of strings of archive.org URLs and any errors.
func GetLatestURLs(urls []string, archiveIfNotFound bool) (archiveUrls []string, errs []error) {
	var errors []error
	var response []string

	for _, url := range urls {
		var err error
		archiveUrl, exists, err := GetLatestURL(url)
		if err != nil {
			errors = append(errors, fmt.Errorf("unable to get latest archive URL for %v, we got: %v, err: %w", url, archiveUrl, err))
			continue
		}
		if !exists && archiveIfNotFound {
			archiveUrl, err = ArchiveURL(url)
			if err != nil {
				errors = append(errors, fmt.Errorf("unable to archive URL %v, we got: %v, err: %w", url, archiveUrl, err))
			}
		}
		response = append(response, archiveUrl)
	}

	return response, errors
}

// Archives a given URL with archive.org. Returns an empty string and an error
// if the URL wasn't archived.
func ArchiveURL(url string) (string, error) {
	http := http.Client{}
	resp, err := http.Get(archiveApi + "/save/" + url)
	if err != nil {
		return "", fmt.Errorf("error calling archive.org: %w", err)
	}

	switch resp.StatusCode {
	case 301, 302:
		// Case insensitive
		location := resp.Header.Get("location")
		if location == "" {
			err = fmt.Errorf("archive.org did not reply with a location header")
		}
		return location, err
	case 523, 520:
		return "", fmt.Errorf("archive.org declined to archive that page")
	default:
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("unable to read response body, err: %v", err)
		}

		// Check if the body has a canonical link in it as a last resort
		canonicalMatch, _ := regexp.Compile(canonicalSearch + "[^\"]+")
		canonicalUrlMatch := canonicalMatch.Find(body)
		canonicalUrl := strings.Split(string(canonicalUrlMatch), canonicalSearch)[1]
		if canonicalUrl != "" {
			return string(canonicalUrl), nil
		}
		return "", fmt.Errorf("archive.org had an unexpected response: %v", string(body))

	}
}
