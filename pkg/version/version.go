package version

// Current defines the application version.
// It defaults to "dev" but is overwritten by the Makefile using -ldflags.
var Current = "dev"

// BuildMetadata can be injected via ldflags if needed, but for now we keep it simple.
const AppName = "CloudSlash"
const License = "AGPLv3 (Enterprise)"
