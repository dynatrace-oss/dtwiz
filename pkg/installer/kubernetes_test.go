package installer

import (
	"strings"
	"testing"
)

func TestRenderDynakubeTemplate_PinnedImages(t *testing.T) {
	data := dynakubeTemplateData{
		ClusterName:      "test-cluster",
		APIURL:           "https://abc123.live.dynatracelabs.com/api",
		APIToken:         "dt0c01.token",
		DataIngestToken:  "dt0c01.token",
		ActiveGateImage:  dynakubeActiveGateImage,
		EECRepository:    dynakubeEECRepository,
		EECTag:           dynakubeEECTag,
		CodeModulesImage: dynakubeCodeModulesImage,
	}

	out, err := renderDynakubeTemplate(data)
	if err != nil {
		t.Fatalf("renderDynakubeTemplate: %v", err)
	}

	checks := []struct {
		desc string
		want string
	}{
		{"ActiveGate image pinned", "image: " + dynakubeActiveGateImage},
		{"EEC repository", "repository: " + dynakubeEECRepository},
		{"EEC tag", "tag: " + dynakubeEECTag},
		{"codeModulesImage pinned", "codeModulesImage: " + dynakubeCodeModulesImage},
	}

	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: rendered manifest does not contain %q", c.desc, c.want)
		}
	}
}

func TestRenderDynakubeTemplate_NoPlaceholders(t *testing.T) {
	data := dynakubeTemplateData{
		ClusterName:      "test-cluster",
		APIURL:           "https://abc123.live.dynatracelabs.com/api",
		APIToken:         "dt0c01.token",
		DataIngestToken:  "dt0c01.token",
		ActiveGateImage:  dynakubeActiveGateImage,
		EECRepository:    dynakubeEECRepository,
		EECTag:           dynakubeEECTag,
		CodeModulesImage: dynakubeCodeModulesImage,
	}

	out, err := renderDynakubeTemplate(data)
	if err != nil {
		t.Fatalf("renderDynakubeTemplate: %v", err)
	}

	if strings.Contains(out, "{{") || strings.Contains(out, "}}") {
		t.Error("rendered manifest still contains unresolved template placeholders")
	}
}
