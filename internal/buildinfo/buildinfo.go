package buildinfo

import "runtime/debug"

type Info struct {
	Version  string `json:"version"`
	Commit   string `json:"commit"`
	Time     string `json:"time,omitempty"`
	Modified bool   `json:"modified"`
}

func Get(version string) Info {
	info := Info{Version: version}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			info.Commit = s.Value
		case "vcs.time":
			info.Time = s.Value
		case "vcs.modified":
			info.Modified = s.Value == "true"
		}
	}
	return info
}
