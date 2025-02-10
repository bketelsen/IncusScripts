package main

import (
	"testing"
)

func Test_orgRepo(t *testing.T) {
	type args struct {
		repo string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
		{"happyPath", args{repo: "https://github.com/bketelsen/IncusScripts"}, "bketelsen/IncusScripts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := orgRepo(tt.args.repo); got != tt.want {
				t.Errorf("orgRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_rawURL(t *testing.T) {
	type args struct {
		repo  string
		paths []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"happyPath", args{repo: "github.com/bketelsen/IncusScripts", paths: []string{"test", "test"}}, "https://raw.githubusercontent.com/bketelsen/IncusScripts/refs/heads/main/test/test"},
		{"json", args{repo: "github.com/bketelsen/IncusScripts", paths: []string{"json", "debian.json"}}, "https://raw.githubusercontent.com/bketelsen/IncusScripts/refs/heads/main/json/debian.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rawURL(tt.args.repo, tt.args.paths...); got != tt.want {
				t.Errorf("rawURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_validateDiskSize(t *testing.T) {
	type args struct {
		size string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"1GB", args{size: "1GB"}, false},
		{"1GiB", args{size: "1GiB"}, false},
		{"1TB", args{size: "1TB"}, false},
		{"1TiB", args{size: "1TiB"}, false},
		{"1MB", args{size: "1MB"}, false},
		{"1MiB", args{size: "1MiB"}, false},
		{"1", args{size: "1"}, true},
		{"1MB1", args{size: "1MB1"}, true},
		{"MB: 3", args{size: "MB: 3"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateDiskSize(tt.args.size); (err != nil) != tt.wantErr {
				t.Errorf("validateDiskSize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
