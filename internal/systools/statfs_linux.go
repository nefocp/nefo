//go:build linux

package systools

import "syscall"

// diskFree returns the bytes free for root and the file system type of the
// file system that holds dir.
func diskFree(dir string) (uint64, string, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, "", err
	}
	return st.Bfree * uint64(st.Bsize), fsName(int64(st.Type)), nil
}

// fsName names the file systems that matter for a swap file.
func fsName(magic int64) string {
	switch uint32(magic) {
	case 0xEF53:
		return "ext4"
	case 0x58465342:
		return "xfs"
	case 0x9123683E:
		return "btrfs"
	case 0x2FC12FC1:
		return "zfs"
	case 0x794C7630:
		return "overlay"
	case 0x01021994:
		return "tmpfs"
	case 0xF2F52010:
		return "f2fs"
	}
	return "other"
}
