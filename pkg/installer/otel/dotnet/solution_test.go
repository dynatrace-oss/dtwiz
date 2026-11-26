package dotnet_test

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

const slnHeader = "\nMicrosoft Visual Studio Solution File, Format Version 12.00\n"

func TestReadSolutionMembers(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		content  string
		want     []string // relative to the solution dir
		wantErr  bool
		wantNone bool
	}{
		{
			name: "sln with windows separators",
			file: "App.sln",
			content: slnHeader + `Project("{FAE04EC0-301F-4F8D-A5B2-0C8B1E3C8B3F}") = "Api", "src\Api\Api.csproj", "{1}"
EndProject
Project("{2150E333-8FDC-42A3-9474-1A3956D46DE8}") = "src", "src", "{2}"
EndProject
`,
			want: []string{"src/Api/Api.csproj"},
		},
		{
			name:    "slnx with folders",
			file:    "App.slnx",
			content: `<Solution><Folder Name="/src/"><Project Path="src/Api/Api.csproj" /></Folder><Project Path="src\Worker\Worker.fsproj" /></Solution>`,
			want:    []string{"src/Api/Api.csproj", "src/Worker/Worker.fsproj"},
		},
		{
			name:     "member outside the solution directory is dropped",
			file:     "App.sln",
			content:  slnHeader + `Project("{F}") = "Out", "..\Other\Other.csproj", "{1}"` + "\nEndProject\n",
			wantNone: true,
		},
		{
			name:     "readable solution without projects",
			file:     "Empty.sln",
			content:  slnHeader,
			wantNone: true,
		},
		{name: "sln without header is unreadable", file: "Bad.sln", content: "garbage", wantErr: true},
		{name: "malformed slnx is unreadable", file: "Bad.slnx", content: "<Solution><Project", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			path := writeFile(t, dir, tt.file, tt.content)

			// Act
			got, err := dotnet.ReadSolutionMembers(path)

			// Assert
			if tt.wantErr {
				if !errors.Is(err, dotnet.ErrSolutionUnreadable) {
					t.Fatalf("err = %v, want ErrSolutionUnreadable", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := make([]string, 0, len(tt.want))
			for _, w := range tt.want {
				want = append(want, filepath.Join(dir, filepath.FromSlash(w)))
			}
			if tt.wantNone && len(got) != 0 {
				t.Errorf("members = %v, want none", got)
			}
			if !tt.wantNone && !reflect.DeepEqual(got, want) {
				t.Errorf("members = %v, want %v", got, want)
			}
		})
	}
}

func TestReadSolutionMembers_MissingFile(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "Missing.sln")

	// Act
	_, err := dotnet.ReadSolutionMembers(path)

	// Assert
	if !errors.Is(err, dotnet.ErrSolutionUnreadable) {
		t.Errorf("err = %v, want ErrSolutionUnreadable", err)
	}
}
