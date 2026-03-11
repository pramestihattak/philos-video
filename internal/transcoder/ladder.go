package transcoder

// Profile defines a single encoding ladder rung.
type Profile struct {
	Name         string
	Width, Height int
	VideoBitrate  string
	AudioBitrate  string
	MaxRate       string
	BufSize       string
}

var defaultLadder = []Profile{
	{
		Name:         "1080p",
		Width:        1920,
		Height:       1080,
		VideoBitrate: "5000k",
		AudioBitrate: "192k",
		MaxRate:      "5350k",
		BufSize:      "7500k",
	},
	{
		Name:         "720p",
		Width:        1280,
		Height:       720,
		VideoBitrate: "2500k",
		AudioBitrate: "128k",
		MaxRate:      "2675k",
		BufSize:      "3750k",
	},
	{
		Name:         "480p",
		Width:        854,
		Height:       480,
		VideoBitrate: "1000k",
		AudioBitrate: "128k",
		MaxRate:      "1070k",
		BufSize:      "1500k",
	},
	{
		Name:         "360p",
		Width:        640,
		Height:       360,
		VideoBitrate: "400k",
		AudioBitrate: "96k",
		MaxRate:      "428k",
		BufSize:      "600k",
	},
}

// BuildLadder returns profiles whose short-side target does not exceed the source's short side.
// This correctly handles both landscape and portrait sources.
func BuildLadder(sourceWidth, sourceHeight int) []Profile {
	sourceShortSide := sourceWidth
	if sourceHeight < sourceShortSide {
		sourceShortSide = sourceHeight
	}
	var result []Profile
	for _, p := range defaultLadder {
		// p.Height is the short-side target for all profiles (e.g. 1080, 720, 480, 360).
		if p.Height <= sourceShortSide {
			result = append(result, p)
		}
	}
	return result
}
