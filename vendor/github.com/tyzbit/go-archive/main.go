package archiveorg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/avast/retry-go"
)

const (
	archiveApi          string = "https://wwwb-api.archive.org"
	archiveRoot         string = "https://web.archive.org/web"
	pendingRetryAttemps uint   = 40
)

type ArchiveOrgWaybackAvailableResponse struct {
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

type ArchiveOrgWaybackSaveResponse struct {
	URL       string `json:"url"`
	JobID     string `json:"job_id"`
	Message   string `json:"message"`
	Status    string `json:"status,omitempty"`
	StatusExt string `json:"statux_ext,omitempty"`
}

type ArchiveOrgWaybackStatusResponse struct {
	Counters struct {
		Embeds   int `json:"embeds"`
		Outlinks int `json:"outlinks"`
	} `json:"counters"`
	DurationSec  float32  `json:"duration_sec"`
	FirstArchive bool     `json:"first_archive"`
	HttpStatus   int      `json:"http_status"`
	JobID        string   `json:"job_id"`
	OriginalURL  string   `json:"original_url"`
	Outlinks     []string `json:"outlinks"`
	Resources    []string `json:"resources"`
	Status       string   `json:"status"`
	Timestamp    string   `json:"timestamp"`
}

type ArchiveOrgWaybackSparklineResponse struct {
	Years   map[string][]int  `json:"years"`
	FirstTs string            `json:"first_ts"`
	LastTs  string            `json:"last_ts"`
	Status  map[string]string `json:"status"`
}

type RetriableError struct {
	Err        error
	RetryAfter time.Duration
}

// Error returns error message and a Retry-After duration.
func (e *RetriableError) Error() string {
	return fmt.Sprintf("%s (retry after %v)", e.Err.Error(), e.RetryAfter)
}

// Checks if a page is available in the Wayback Machine.
// r.ArchivedSnapshots will be populated if it is.
func CheckURLWaybackAvailable(url string, retryAttempts uint) (r ArchiveOrgWaybackAvailableResponse, err error) {
	resp := http.Response{}
	if err := retry.Do(func() error {
		client := http.Client{}
		respTry, err := client.Get(archiveApi + "/wayback/available?url=" + url)
		if err != nil {
			return &RetriableError{
				Err:        fmt.Errorf("error calling archive.org wayback api: %w", err),
				RetryAfter: 1 * time.Second,
			}
		}
		if resp.StatusCode == 429 {
			return fmt.Errorf("rate limited by archive.org wayback api")
		}
		resp = *respTry
		return nil
	},
		retry.Attempts(retryAttempts),
		retry.Delay(1*time.Second),
		retry.DelayType(retry.FixedDelay),
	); err != nil {
		// retry returns a pretty human-readable error message
		return r, err
	} else {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				panic(err)
			}
		}()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return r, fmt.Errorf("error reading body from wayback api: %w", err)
		}

		err = json.Unmarshal(body, &r)
		if err != nil {
			return r, fmt.Errorf("error unmarshalling json: %w, body: %v", err, string(body))
		}

		return r, nil
	}
}

// GetLatestUrl returns the latest archive.org link for a given URL.
// Cookie can be blank but then this will only be successful
// if there's an archived page already.
func GetLatestURL(url string, retryAttempts uint, requestArchive bool, cookie string) (latestUrl string, err error) {
	closestURL := ""
	if !requestArchive {
		r, err := CheckURLWaybackAvailable(url, retryAttempts)
		if err != nil {
			return "", fmt.Errorf("error checking if url is available: %w", err)
		}

		closestURL = r.ArchivedSnapshots.Closest.URL
	}

	if closestURL == "" {
		archiveUrl, err := ArchiveURL(url, retryAttempts, cookie)
		if err != nil {
			return "", fmt.Errorf("unable to archive URL: %w", err)
		}
		// At this point, even if the URL is blank we should return it.
		closestURL = archiveUrl
	}

	return closestURL, nil
}

// Takes a slice of strings and a boolean whether or not to archive the page if not found
// and returns a slice of strings of archive.org URLs and any errors.
// Cookie can be blank but then this will only be successful
// if there's an archived page already.
func GetLatestURLs(urls []string, retryAttempts uint, requestArchive bool, cookie string) (archiveUrls []string, errs []error) {
	for _, url := range urls {
		var err error
		archiveUrl, err := GetLatestURL(url, retryAttempts, requestArchive, cookie)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		archiveUrls = append(archiveUrls, archiveUrl)
	}

	return archiveUrls, errs
}

// Archives a given URL with archive.org. Returns an empty string and an error
// if the URL wasn't archived.
// Needs authentication (cookie).
func ArchiveURL(archiveURL string, retryAttempts uint, cookie string) (archivedURL string, err error) {
	urlSnapshot := ""
	if err := retry.Do(func() error {
		client := &http.Client{}
		urlParams := "capture_all=1&url=" + url.QueryEscape(archiveURL)
		r, err := http.NewRequest(http.MethodPost, archiveApi+"/save/?"+urlParams, bytes.NewBuffer([]byte(urlParams)))
		if err != nil {
			return fmt.Errorf("Could not build http request")
		}
		r.Header = http.Header{
			"Accept":       {"application/json"},
			"Content-Type": {"application/x-www-form-urlencoded"},
			"Cookie":       {cookie},
		}
		resp, err := client.Do(r)
		if err != nil {
			return &RetriableError{
				Err:        fmt.Errorf("error calling archive.org: %w", err),
				RetryAfter: 3 * time.Second,
			}
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				panic(err)
			}
		}()

		switch resp.StatusCode {
		// May not be necessary anymore now that we're calling a real API
		case 301, 302:
			// Case insensitive
			location := resp.Header.Get("location")
			if location == "" {
				return &RetriableError{
					Err:        fmt.Errorf("archive.org did not reply with a location header"),
					RetryAfter: 3 * time.Second,
				}
			} else {
				urlSnapshot = location
			}
			return err
		// May not be necessary anymore now that we're calling a real API
		case 523, 520:
			return fmt.Errorf("archive.org declined to archive the page")
		default:
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("unable to read response body, err: %v", err)
			}

			s := ArchiveOrgWaybackSaveResponse{}
			_ = json.Unmarshal(body, &s)
			if s.JobID == "" {
				var message string
				if s.Message != "" {
					message = s.Message
				} else {
					message = string(body)
				}
				return &RetriableError{
					Err:        fmt.Errorf("archive.org did not respond with a job_id: %v", message),
					RetryAfter: 3 * time.Second,
				}
			}

			rs, err := CheckArchiveRequestStatus(s.JobID)
			if err != nil {
				return &RetriableError{
					Err:        fmt.Errorf("error checking archive request status: %v", string(body)),
					RetryAfter: 3 * time.Second,
				}
			}

			// Retry if pending
			if rs.Status == "pending" {
				if err := retry.Do(func() error {
					rs, err = CheckArchiveRequestStatus(s.JobID)
					if rs.Status == "success" {
						return nil
					}
					return &RetriableError{
						Err:        fmt.Errorf("job is still pending"),
						RetryAfter: 3 * time.Second,
					}
				},
					retry.Attempts(pendingRetryAttemps),
					retry.Delay(1*time.Second),
					retry.DelayType(retry.BackOffDelay),
				); err != nil {
					// retry returns a pretty human-readable error message
					return err
				}
			}

			if rs.Status != "success" {
				return fmt.Errorf("archive.org request had unexpected status: %v", rs.Status)
			}

			// The job returned success
			if rs.Timestamp != "" {
				// We could call the archive.org API again
				// but URLs are predictable
				urlSnapshot = archiveRoot + rs.Timestamp + archiveURL
				return nil
			}
		}
		return &RetriableError{
			Err:        fmt.Errorf("archive.org had unexpected http status code: %v", resp.StatusCode),
			RetryAfter: 3 * time.Second,
		}
	},
		retry.Attempts(retryAttempts),
		retry.Delay(1*time.Second),
		retry.DelayType(retry.BackOffDelay),
	); err != nil {
		// retry returns a pretty human-readable error message
		return "", err
	}

	// This should always be a successful response.
	return urlSnapshot, err
}

// Checks the status of an archive request job.
func CheckArchiveRequestStatus(jobID string) (r ArchiveOrgWaybackStatusResponse, err error) {
	client := http.Client{}
	resp, err := client.Get(archiveApi + "/save/status/" + jobID)
	if err != nil {
		return r, fmt.Errorf("error calling archive.org status api: %w", err)
	}
	if resp.StatusCode == 429 {
		return r, fmt.Errorf("rate limited by archive.org status api")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return r, fmt.Errorf("error reading body: %w", err)
	}
	err = json.Unmarshal(body, &r)
	if err != nil {
		return r, fmt.Errorf("error unmarshalling json: %w, body: %v", err, string(body))
	}
	return r, nil
}

// Checks the sparkline (history of archived copies) for a given URL.
// Does not need to be authenticated.
func CheckArchiveSparkline(url string) (r ArchiveOrgWaybackSparklineResponse, err error) {
	client := http.Client{}
	resp, err := client.Get(archiveApi + "/__wb/sparkline/?collection=web&output=json&url=" + url)
	if err != nil {
		return r, fmt.Errorf("error calling archive.org sparkline api: %w", err)
	}
	if resp.StatusCode == 429 {
		return r, fmt.Errorf("rate limited by archive.org sparkline api")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return r, fmt.Errorf("error reading body: %w", err)
	}
	err = json.Unmarshal(body, &r)
	if err != nil {
		return r, fmt.Errorf("error unmarshalling json: %w, body: %v", err, string(body))
	}
	return r, nil
}
