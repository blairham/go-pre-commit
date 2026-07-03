package languages

import "testing"

func TestContainerIDFromMountinfo(t *testing.T) {
	tests := []struct {
		name      string
		mountinfo string
		want      string
	}{
		{
			name: "docker cgroups v2",
			mountinfo: `1014 1013 0:107 / / rw,relatime master:462 - overlay overlay rw,lowerdir=/var/lib/docker/overlay2/l/UY,upperdir=/var/lib/docker/overlay2/dd/diff
1023 1014 254:1 /docker/containers/c33988ec7651ebc867cb24755eaf637a6734088bc7eae570d1a3116a1c3d9dea/hostname /etc/hostname rw,relatime - ext4 /dev/vda1 rw`,
			want: "c33988ec7651ebc867cb24755eaf637a6734088bc7eae570d1a3116a1c3d9dea",
		},
		{
			name:      "podman overlay-containers",
			mountinfo: `1023 1014 254:1 /containers/overlay-containers/c33988ec7651ebc867cb24755eaf637a6734088bc7eae570d1a3116a1c3d9dea/userdata/hostname /etc/hostname rw,relatime - ext4 /dev/vda1 rw`,
			want:      "c33988ec7651ebc867cb24755eaf637a6734088bc7eae570d1a3116a1c3d9dea",
		},
		{
			name:      "not in a container",
			mountinfo: `26 30 0:5 / /dev rw,nosuid,relatime shared:2 - devtmpfs udev rw,size=8022368k`,
			want:      "",
		},
		{
			name:      "short id does not match",
			mountinfo: `1023 1014 254:1 /docker/containers/c33988ec7651/hostname /etc/hostname rw - ext4 /dev/vda1 rw`,
			want:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containerIDFromMountinfo([]byte(tt.mountinfo)); got != tt.want {
				t.Errorf("containerIDFromMountinfo() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTranslateMountPath(t *testing.T) {
	inspect := `[{"Mounts": [
		{"Source": "/home/user/project", "Destination": "/src"},
		{"Source": "/var/run/docker.sock", "Destination": "/var/run/docker.sock"}
	]}]`
	tests := []struct {
		name    string
		path    string
		inspect string
		want    string
	}{
		{"exact match", "/src", inspect, "/home/user/project"},
		{"subpath", "/src/sub/dir", inspect, "/home/user/project/sub/dir"},
		{"not mounted", "/tmp/elsewhere", inspect, "/tmp/elsewhere"},
		{"prefix but not path boundary", "/srcfoo", inspect, "/srcfoo"},
		{"invalid json", "/src", `not json`, "/src"},
		{"empty array", "/src", `[]`, "/src"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := translateMountPath(tt.path, []byte(tt.inspect)); got != tt.want {
				t.Errorf("translateMountPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestRootlessFromInfo(t *testing.T) {
	tests := []struct {
		name string
		info string
		want bool
	}{
		{"rootful docker", `{"SecurityOptions": ["name=apparmor", "name=seccomp,profile=builtin"]}`, false},
		{"rootless docker", `{"SecurityOptions": ["name=seccomp,profile=builtin", "name=rootless"]}`, true},
		{"null security options", `{"SecurityOptions": null}`, false},
		{"rootless podman", `{"host": {"security": {"rootless": true}}}`, true},
		{"rootful podman", `{"host": {"security": {"rootless": false}}}`, false},
		{"invalid json", `oops`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rootlessFromInfo([]byte(tt.info)); got != tt.want {
				t.Errorf("rootlessFromInfo(%s) = %v, want %v", tt.info, got, tt.want)
			}
		})
	}
}
