package kernel

import (
	"context"
	"debug/elf"
	"fmt"
	"strings"

	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/file"
	"github.com/anchore/syft/syft/internal/unionreader"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/pkg/cataloger/generic"
)

const modinfoName = ".modinfo"

func parseLinuxKernelModuleFile(ctx context.Context, _ file.Resolver, _ *generic.Environment, reader file.LocationReadCloser) ([]pkg.Package, []artifact.Relationship, error) {
	unionReader, err := unionreader.GetUnionReader(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get union reader for file: %w", err)
	}
	metadata, err := parseLinuxKernelModuleMetadata(unionReader)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse kernel module metadata: %w", err)
	}
	if metadata == nil || metadata.KernelVersion == "" {
		return nil, nil, nil
	}

	metadata.Path = reader.RealPath

	return []pkg.Package{
		newLinuxKernelModulePackage(
			ctx,
			*metadata,
			reader.Location,
		),
	}, nil, nil
}

func parseLinuxKernelModuleMetadata(r unionreader.UnionReader) (p *pkg.LinuxKernelModule, err error) {
	// filename:       /lib/modules/5.15.0-1031-aws/kernel/zfs/zzstd.ko
	// version:        1.4.5a
	// license:        Dual BSD/GPL
	// description:    ZSTD Compression for ZFS
	// srcversion:     F1F818A6E016499AB7F826E
	// depends:        spl
	// retpoline:      Y
	// name:           zzstd
	// vermagic:       5.15.0-1031-aws SMP mod_unload modversions
	// sig_id:         PKCS#7
	// signer:         Build time autogenerated kernel key
	// sig_key:        49:A9:55:87:90:5B:33:41:AF:C0:A7:BE:2A:71:6C:D2:CA:34:E0:AE
	// sig_hashalgo:   sha512
	//
	// OR
	//
	// filename:       /home/ubuntu/eve/rootfs/lib/modules/5.10.121-linuxkit/kernel/drivers/net/wireless/realtek/rtl8821cu/8821cu.ko
	// version:        v5.4.1_28754.20180921_COEX20180712-3232
	// author:         Realtek Semiconductor Corp.
	// description:    Realtek Wireless Lan Driver
	// license:        GPL
	// srcversion:     960CCC648A0E0369171A2C9
	// depends:        cfg80211
	// retpoline:      Y
	// name:           8821cu
	// vermagic:       5.10.121-linuxkit SMP mod_unload
	p = &pkg.LinuxKernelModule{
		Parameters: make(map[string]pkg.LinuxKernelModuleParameter),
	}
	f, err := elf.NewFile(r)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	modinfo := f.Section(modinfoName)
	if modinfo == nil {
		return nil, fmt.Errorf("no section %s", modinfoName)
	}
	b, err := modinfo.Data()
	if err != nil {
		return nil, fmt.Errorf("error reading secion %s: %w", modinfoName, err)
	}
	var (
		entry []byte
	)
	for _, b2 := range b {
		if b2 == 0 {
			if err := addLinuxKernelModuleEntry(p, entry); err != nil {
				return nil, fmt.Errorf("error parsing entry %s: %w", string(entry), err)
			}
			entry = []byte{}
			continue
		}
		entry = append(entry, b2)
	}
	if err := addLinuxKernelModuleEntry(p, entry); err != nil {
		return nil, fmt.Errorf("error parsing entry %s: %w", string(entry), err)
	}

	return p, nil
}

func addLinuxKernelModuleEntry(k *pkg.LinuxKernelModule, entry []byte) error {
	if len(entry) == 0 {
		return nil
	}
	var key, value string
	parts := strings.SplitN(string(entry), "=", 2)
	if len(parts) > 0 {
		key = parts[0]
	}
	if len(parts) > 1 {
		value = parts[1]
	}

	switch key {
	case "version":
		k.Version = value
	case "license":
		k.License = value
	case "author":
		k.Author = value
	case "name":
		k.Name = value
	case "vermagic":
		k.VersionMagic = value
		fields := strings.Fields(value)
		if len(fields) > 0 {
			k.KernelVersion = fields[0]
		}
	case "srcversion":
		k.SourceVersion = value
	case "description":
		k.Description = value
	case "parm":
		parts := strings.SplitN(value, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid parm entry: %s", value)
		}
		if m, ok := k.Parameters[parts[0]]; !ok {
			k.Parameters[parts[0]] = pkg.LinuxKernelModuleParameter{Description: parts[1]}
		} else {
			m.Description = parts[1]
		}
	case "parmtype":
		parts := strings.SplitN(value, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid parmtype entry: %s", value)
		}
		if m, ok := k.Parameters[parts[0]]; !ok {
			k.Parameters[parts[0]] = pkg.LinuxKernelModuleParameter{Type: parts[1]}
		} else {
			m.Type = parts[1]
		}
	}
	return nil
}
