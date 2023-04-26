//go:build sync
// +build sync

package extensions

import (
	"context"

	"zotregistry.io/zot/pkg/api/config"
	"zotregistry.io/zot/pkg/extensions/sync"
	"zotregistry.io/zot/pkg/log"
	"zotregistry.io/zot/pkg/meta/repodb"
	"zotregistry.io/zot/pkg/storage"
)

func EnableSyncExtension(ctx context.Context, config *config.Config,
	repoDB repodb.RepoDB, storeController storage.StoreController, log log.Logger,
) {
	if config.Extensions.Sync != nil && *config.Extensions.Sync.Enable {
		if err := sync.Run(ctx, *config.Extensions.Sync, repoDB, storeController, log); err != nil {
			log.Error().Err(err).Msg("Error encountered while setting up syncing")
		}
	} else {
		log.Info().Msg("Sync registries config not provided or disabled, skipping sync")
	}
}

func SyncOneImage(ctx context.Context, config *config.Config, repoDB repodb.RepoDB,
	storeController storage.StoreController, repoName, reference string, artifactType string, log log.Logger,
) error {
	log.Info().Str("repository", repoName).Str("reference", reference).Msg("syncing image")

	err := sync.OneImage(ctx, *config.Extensions.Sync, repoDB, storeController, repoName, reference, artifactType, log)

	return err
}
