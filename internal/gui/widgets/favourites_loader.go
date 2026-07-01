package widgets

import (
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

// LoadEntryFormFavourites reads the user's configured favourite associations from
// storage and converts them to the EntryFormFavourite slice consumed by
// ShowEntryFormDialog's quick-pick. Returns nil on any error so callers can pass
// the result straight through without checking — the dialog simply renders without
// favourites in that case.
func LoadEntryFormFavourites() []EntryFormFavourite {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil
	}
	store, err := storage.New(cfg.GetStorageDir())
	if err != nil {
		return nil
	}
	favs, err := store.LoadFavouriteAssociations()
	if err != nil || favs == nil {
		return nil
	}
	out := make([]EntryFormFavourite, 0, len(favs.Associations))
	for _, fav := range favs.Associations {
		if fav.ProjectID <= 0 {
			continue
		}
		out = append(out, EntryFormFavourite{
			ProjectID:    fav.ProjectID,
			ProjectName:  fav.ProjectName,
			TaskID:       fav.TaskID,
			TaskName:     fav.TaskName,
			ActivityID:   fav.ActivityID,
			ActivityName: fav.ActivityName,
		})
	}
	return out
}
