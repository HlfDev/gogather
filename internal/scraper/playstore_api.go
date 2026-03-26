package scraper

// PlayStoreAPIScraper fetches reviews from the official Google Play Developer API v3.
//
// Authentication uses a Google Cloud service account (JWT bearer token, no external
// dependencies — implemented with Go's standard crypto libraries).
//
// API reference:
//
//	GET https://androidpublisher.googleapis.com/androidpublisher/v3/applications/{package}/reviews
//	Scope: https://www.googleapis.com/auth/androidpublisher
//
// Reviews are returned sorted by lastModified descending (most recently submitted
// or edited first), which is the correct order for a monitoring use case.

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	androidPublisherScope   = "https://www.googleapis.com/auth/androidpublisher"
	reviewsAPIURL           = "https://androidpublisher.googleapis.com/androidpublisher/v3/applications/%s/reviews"
	defaultTokenURI         = "https://oauth2.googleapis.com/token"
	jwtGrantType            = "urn:ietf:params:oauth:grant-type:jwt-bearer"
	reviewsPerPage          = 100
)

// serviceAccountKey holds the fields we need from a Google service account JSON key file.
type serviceAccountKey struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

type cachedToken struct {
	mu      sync.Mutex
	value   string
	expires time.Time
}

// PlayStoreAPIScraper fetches Play Store reviews via the Google Play Developer API.
type PlayStoreAPIScraper struct {
	PackageName string
	Lang        string
	Country     string
	key         *serviceAccountKey
	token       cachedToken
	client      *http.Client
}

// NewPlayStoreAPIScraper parses credentialsJSON (contents of a service account key file)
// and returns a scraper ready to use.
func NewPlayStoreAPIScraper(packageName, lang, country, credentialsJSON string) (*PlayStoreAPIScraper, error) {
	var key serviceAccountKey
	if err := json.Unmarshal([]byte(credentialsJSON), &key); err != nil {
		return nil, fmt.Errorf("parse service account credentials: %w", err)
	}
	if key.ClientEmail == "" || key.PrivateKey == "" {
		return nil, fmt.Errorf("service account JSON is missing client_email or private_key")
	}
	if key.TokenURI == "" {
		key.TokenURI = defaultTokenURI
	}
	return &PlayStoreAPIScraper{
		PackageName: packageName,
		Lang:        lang,
		Country:     country,
		key:         &key,
		client:      &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// FetchReviews fetches the most recent reviews via the Developer API.
// Reviews are returned sorted by lastModified descending (newest first).
func (s *PlayStoreAPIScraper) FetchReviews() ([]Review, error) {
	appName := fetchPlayStoreAppName(s.PackageName, s.Lang, s.Country, s.client)

	token, err := s.accessToken()
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	return s.listReviews(token, appName)
}

// accessToken returns a cached Bearer token, refreshing it when it is about to expire.
func (s *PlayStoreAPIScraper) accessToken() (string, error) {
	s.token.mu.Lock()
	defer s.token.mu.Unlock()

	if s.token.value != "" && time.Now().Before(s.token.expires) {
		return s.token.value, nil
	}

	jwt, err := s.makeJWT()
	if err != nil {
		return "", err
	}

	body := url.Values{
		"grant_type": {jwtGrantType},
		"assertion":  {jwt},
	}.Encode()

	resp, err := s.client.Post(s.key.TokenURI, "application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("token response parse: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("token error %q: %s", result.Error, result.ErrorDesc)
	}

	// Expire 60 s early to avoid using a token that is about to become invalid.
	s.token.value = result.AccessToken
	s.token.expires = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return s.token.value, nil
}

// makeJWT builds and signs a JWT for the service account OAuth2 flow.
func (s *PlayStoreAPIScraper) makeJWT() (string, error) {
	block, _ := pem.Decode([]byte(s.key.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("private key: invalid PEM block")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("private key: %w", err)
	}
	rsaKey, ok := priv.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("private key is not RSA")
	}

	now := time.Now().Unix()

	headerJSON, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claimsJSON, _ := json.Marshal(map[string]interface{}{
		"iss":   s.key.ClientEmail,
		"scope": androidPublisherScope,
		"aud":   s.key.TokenURI,
		"iat":   now,
		"exp":   now + 3600,
	})

	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	claims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := header + "." + claims

	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("JWT sign: %w", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// listReviews pages through the API until it has fetched up to reviewsPerPage reviews.
func (s *PlayStoreAPIScraper) listReviews(token, appName string) ([]Review, error) {
	apiURL := fmt.Sprintf(reviewsAPIURL, s.PackageName)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("maxResults", strconv.Itoa(reviewsPerPage))
	if s.Lang != "" {
		q.Set("translationLanguage", s.Lang)
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reviews list: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reviews list status %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		Reviews []struct {
			ReviewID   string `json:"reviewId"`
			AuthorName string `json:"authorName"`
			Comments   []struct {
				UserComment *struct {
					Text           string `json:"text"`
					LastModified   *struct {
						Seconds string `json:"seconds"`
					} `json:"lastModified"`
					StarRating     int    `json:"starRating"`
					AppVersionName string `json:"appVersionName"`
				} `json:"userComment"`
			} `json:"comments"`
		} `json:"reviews"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("reviews parse: %w", err)
	}

	var reviews []Review
	for _, r := range result.Reviews {
		if len(r.Comments) == 0 || r.Comments[0].UserComment == nil {
			continue
		}
		uc := r.Comments[0].UserComment

		var date time.Time
		if uc.LastModified != nil && uc.LastModified.Seconds != "" {
			if ts, err := strconv.ParseInt(uc.LastModified.Seconds, 10, 64); err == nil {
				date = time.Unix(ts, 0)
			}
		}

		reviews = append(reviews, Review{
			ID:      r.ReviewID,
			Source:  SourcePlayStore,
			AppName: appName,
			Author:  r.AuthorName,
			Rating:  uc.StarRating,
			Body:    uc.Text,
			Date:    date,
			Version: uc.AppVersionName,
		})
	}
	return reviews, nil
}
