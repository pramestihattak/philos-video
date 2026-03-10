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

// BuildLadder returns profiles whose resolution does not exceed the source dimensions.
func BuildLadder(sourceWidth, sourceHeight int) []Profile {
	var result []Profile
	for _, p := range defaultLadder {
		if p.Width <= sourceWidth && p.Height <= sourceHeight {
			result = append(result, p)
		}
	}
	return result
}
