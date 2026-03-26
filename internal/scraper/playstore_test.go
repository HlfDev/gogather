package scraper

import (
	"fmt"
	"sort"
	"testing"
)

func TestPlayStoreFetchReviews(t *testing.T) {
	s := NewPlayStoreScraper("br.com.dafiti", "pt", "br")

	reviews, err := s.FetchReviews()
	if err != nil {
		t.Fatalf("FetchReviews error: %v", err)
	}
	if len(reviews) == 0 {
		t.Fatal("nenhuma review retornada")
	}

	t.Logf("total de reviews retornadas: %d", len(reviews))

	// Simula o sort feito em main.go (mais antiga primeiro)
	sort.Slice(reviews, func(i, j int) bool {
		return reviews[i].Date.Before(reviews[j].Date)
	})

	for i, r := range reviews {
		body := r.Body
		if len(body) > 60 {
			body = body[:60] + "…"
		}
		t.Logf("[%02d] %s | %d★ | v%-8s | %s | %s",
			i+1,
			r.Date.Format("2006-01-02 15:04"),
			r.Rating,
			r.Version,
			r.Author,
			body,
		)
	}

	// Valida campos obrigatórios
	for i, r := range reviews {
		if r.ID == "" {
			t.Errorf("[%d] review sem ID", i+1)
		}
		if r.Rating < 1 || r.Rating > 5 {
			t.Errorf("[%d] rating inválido: %d", i+1, r.Rating)
		}
		if r.Date.IsZero() {
			t.Errorf("[%d] data ausente", i+1)
		}
		if r.Body == "" {
			t.Errorf("[%d] body vazio", i+1)
		}
	}

	// Confirma a mais recente (que vai ao Slack por último = mais visível)
	most := reviews[len(reviews)-1]
	fmt.Printf("\n--- review mais recente (última no Slack) ---\n")
	fmt.Printf("Data:  %s\nAutor: %s\nNota:  %d★\nTexto: %s\n",
		most.Date.Format("2006-01-02 15:04"), most.Author, most.Rating, most.Body)
}
