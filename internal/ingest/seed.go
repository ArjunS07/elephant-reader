package ingest

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"

	"elephant/internal/db"
	"elephant/internal/model"
)

//go:embed seed.json
var seedFS embed.FS

type seedFile struct {
	Sources    []model.Source    `json:"sources"`
	Items      []seedItem        `json:"items"`
	Highlights []model.Highlight `json:"highlights"`
}

type seedItem struct {
	model.IngestItem
	full    model.UserItemState
	hasFull bool
}

func (s *seedItem) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &s.IngestItem); err != nil {
		return err
	}
	var raw struct {
		State *struct {
			InLater      bool    `json:"inLater"`
			SavedAt      *string `json:"savedAt"`
			ReadAt       *string `json:"readAt"`
			ReadProgress float64 `json:"readProgress"`
		} `json:"initialState"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.State != nil {
		inLater := raw.State.InLater
		s.InitialState = &model.InitialState{InLater: &inLater, ReadAt: raw.State.ReadAt}
		s.full = model.UserItemState{
			InLater:      raw.State.InLater,
			SavedAt:      raw.State.SavedAt,
			ReadAt:       raw.State.ReadAt,
			ReadProgress: raw.State.ReadProgress,
		}
		s.hasFull = true
	}
	return nil
}

func Seed(ctx context.Context, store *db.Store) error {
	body, err := seedFS.ReadFile("seed.json")
	if err != nil {
		return err
	}
	var file seedFile
	if err := json.Unmarshal(body, &file); err != nil {
		return err
	}
	svc := New(store)
	taken := map[string]bool{}
	for _, source := range file.Sources {
		if source.SourceType == "newsletter" && source.EmailAlias == "" {
			source.EmailAlias = NewsletterAlias(source.Name, func(alias string) bool {
				if taken[alias] {
					return true
				}
				ok, err := store.AliasTaken(alias)
				return err == nil && ok
			})
			taken[source.EmailAlias] = true
		}
		if _, err := store.UpsertSource(source); err != nil {
			return fmt.Errorf("source %s: %w", source.ID, err)
		}
	}
	for _, item := range file.Items {
		result, err := svc.ingestOne(item.IngestItem)
		if errors.Is(err, errYouTubeShort) {
			continue
		}
		if err != nil {
			return fmt.Errorf("item %s: %w", item.Title, err)
		}
		if result.Created && item.hasFull {
			if err := store.PutState(result.Item.ID, item.full); err != nil {
				return err
			}
		}
	}
	for _, highlight := range file.Highlights {
		if _, err := store.GetHighlight(highlight.ID); err == nil {
			continue
		}
		if _, err := store.AddHighlight(highlight); err != nil {
			return fmt.Errorf("highlight %s: %w", highlight.ID, err)
		}
	}
	return ctx.Err()
}
