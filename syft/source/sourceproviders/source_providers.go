package sourceproviders

import (
	"github.com/anchore/go-collections"
	"github.com/anchore/stereoscope"
	"github.com/anchore/stereoscope/pkg/image"
	"github.com/anchore/syft/syft/source"
	"github.com/anchore/syft/syft/source/directorysource"
	"github.com/anchore/syft/syft/source/filesource"
	"github.com/anchore/syft/syft/source/snapsource"
	"github.com/anchore/syft/syft/source/stereoscopesource"
)

const (
	FileTag = stereoscope.FileTag
	DirTag  = stereoscope.DirTag
	PullTag = stereoscope.PullTag
	SnapTag = "snap"
)

// All returns all the configured source providers known to syft
func All(userInput string, cfg *Config) []collections.TaggedValue[source.Provider] {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	stereoscopeProviders := stereoscopeSourceProviders(userInput, cfg)

	return collections.TaggedValueSet[source.Provider]{}.
		// 1. try all specific, local sources first...

		// --from file, dir, oci-archive, etc.
		Join(stereoscopeProviders.Select(FileTag, DirTag)...).

		// --from snap (local only)
		Join(tagProvider(snapsource.NewLocalSourceProvider(userInput, cfg.Exclude, cfg.DigestAlgorithms, cfg.Alias), SnapTag)).

		// 2. try unspecific, local sources after other local sources last...
		Join(tagProvider(filesource.NewSourceProvider(userInput, cfg.Exclude, cfg.DigestAlgorithms, cfg.Alias), FileTag)).
		Join(tagProvider(directorysource.NewSourceProvider(userInput, cfg.Exclude, cfg.Alias, cfg.BasePath), DirTag)).

		// 3. try remote sources after everything else...

		// --from docker, registry, etc.
		Join(stereoscopeProviders.Select(PullTag)...).

		// --from snap (remote only)
		Join(tagProvider(snapsource.NewRemoteSourceProvider(userInput, cfg.Exclude, cfg.DigestAlgorithms, cfg.Alias), SnapTag))
}

func stereoscopeSourceProviders(userInput string, cfg *Config) collections.TaggedValueSet[source.Provider] {
	var registry image.RegistryOptions
	if cfg.RegistryOptions != nil {
		registry = *cfg.RegistryOptions
	}
	stereoscopeProviders := stereoscopesource.Providers(stereoscopesource.ProviderConfig{
		StereoscopeImageProviderConfig: stereoscope.ImageProviderConfig{
			UserInput: userInput,
			Platform:  cfg.Platform,
			Registry:  registry,
		},
		Alias:   cfg.Alias,
		Exclude: cfg.Exclude,
	})
	return stereoscopeProviders
}

func tagProvider(provider source.Provider, tags ...string) collections.TaggedValue[source.Provider] {
	return collections.NewTaggedValue(provider, append([]string{provider.Name()}, tags...)...)
}
