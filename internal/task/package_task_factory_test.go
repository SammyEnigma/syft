package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/cataloging"
	"github.com/anchore/syft/syft/cpe"
	"github.com/anchore/syft/syft/file"
	"github.com/anchore/syft/syft/pkg"
)

func Test_hasAuthoritativeCPE(t *testing.T) {
	tests := []struct {
		name string
		cpes []cpe.CPE
		want bool
	}{
		{
			name: "no cpes",
			cpes: []cpe.CPE{},
			want: false,
		},
		{
			name: "no authoritative cpes",
			cpes: []cpe.CPE{
				{
					Source: cpe.GeneratedSource,
				},
			},
			want: false,
		},
		{
			name: "has declared (authoritative) cpe",
			cpes: []cpe.CPE{
				{
					Source: cpe.DeclaredSource,
				},
			},
			want: true,
		},
		{
			name: "has lookup (authoritative) cpe",
			cpes: []cpe.CPE{
				{
					Source: cpe.NVDDictionaryLookupSource,
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, hasAuthoritativeCPE(tt.cpes))
		})
	}
}

func TestApplyCompliance(t *testing.T) {
	p1 := pkg.Package{Name: "pkg-1", Version: "1.0"}
	p2 := pkg.Package{Name: "", Version: "1.0"}   // missing name
	p3 := pkg.Package{Name: "pkg-3", Version: ""} // missing version
	p4 := pkg.Package{Name: "pkg-4", Version: ""} // missing version
	c1 := file.Coordinates{RealPath: "/coords/1"}
	c2 := file.Coordinates{RealPath: "/coords/2"}

	for _, p := range []*pkg.Package{&p1, &p2, &p3, &p4} {
		p.SetID()
	}

	r1 := artifact.Relationship{
		From: p1,
		To:   c1,
		Type: artifact.ContainsRelationship,
	}
	r2 := artifact.Relationship{
		From: p2,
		To:   c2,
		Type: artifact.ContainsRelationship,
	}

	cfg := cataloging.ComplianceConfig{
		MissingName:    cataloging.ComplianceActionDrop,
		MissingVersion: cataloging.ComplianceActionStub,
	}

	remainingPkgs, remainingRels := applyCompliance(cfg, []pkg.Package{p1, p2, p3, p4}, []artifact.Relationship{r1, r2})

	// p2 should be dropped because it has a missing name, p3 and p4 should pass with a warning for the missing version
	assert.Len(t, remainingPkgs, 3) // p1, p3, p4 should remain
	assert.Len(t, remainingRels, 1) // only r1 should remain (relationship involving p1)
}

func TestFilterNonCompliantPackages(t *testing.T) {
	p1 := pkg.Package{Name: "pkg-1", Version: "1.0"}
	p2 := pkg.Package{Name: "", Version: "1.0"}   // missing name
	p3 := pkg.Package{Name: "pkg-3", Version: ""} // missing version

	for _, p := range []*pkg.Package{&p1, &p2, &p3} {
		p.SetID()
	}

	cfg := cataloging.ComplianceConfig{
		MissingName:    cataloging.ComplianceActionDrop,
		MissingVersion: cataloging.ComplianceActionKeep,
	}

	remainingPkgs, droppedPkgs, replacement := filterNonCompliantPackages([]pkg.Package{p1, p2, p3}, cfg)
	require.Nil(t, replacement)

	// p2 should be dropped because it has a missing name
	assert.Len(t, remainingPkgs, 2)
	assert.Len(t, droppedPkgs, 1)
	assert.Equal(t, p2, droppedPkgs[0])
}

func TestApplyLicenseContentRules(t *testing.T) {
	licenseWithSPDX := pkg.License{
		SPDXExpression: "MIT",
		Contents:       "MIT license content",
	}
	licenseWithoutSPDX := pkg.License{
		Value:    "License-Not-A-SPDX-Expression",
		Contents: "Non-SPDX license content",
	}

	tests := []struct {
		name             string
		inputLicenses    []pkg.License
		cfg              cataloging.LicenseConfig
		expectedLicenses []pkg.License
	}{
		{
			name: "LicenseContentIncludeUnknown",
			inputLicenses: []pkg.License{
				licenseWithSPDX,
				licenseWithoutSPDX,
			},
			cfg: cataloging.LicenseConfig{
				IncludeContent: cataloging.LicenseContentIncludeUnknown,
			},
			expectedLicenses: []pkg.License{
				{
					SPDXExpression: "MIT",
					Contents:       "", // content cleared for SPDX license
				},
				{
					Value:    "License-Not-A-SPDX-Expression",
					Contents: "Non-SPDX license content", // content preserved for non-SPDX
				},
			},
		},
		{
			name: "LicenseContentExcludeAll",
			inputLicenses: []pkg.License{
				licenseWithSPDX,
				licenseWithoutSPDX,
			},
			cfg: cataloging.LicenseConfig{
				IncludeContent: cataloging.LicenseContentExcludeAll,
			},
			expectedLicenses: []pkg.License{
				{
					SPDXExpression: "MIT",
					Contents:       "", // content cleared
				},
				{
					Value:    "License-Not-A-SPDX-Expression",
					Contents: "", // content cleared
				},
			},
		},
		{
			name: "LicenseContentIncludeAll",
			inputLicenses: []pkg.License{
				licenseWithSPDX,
				licenseWithoutSPDX,
			},
			cfg: cataloging.LicenseConfig{
				IncludeContent: cataloging.LicenseContentIncludeAll,
			},
			expectedLicenses: []pkg.License{
				{
					SPDXExpression: "MIT",
					Contents:       "MIT license content", // content preserved
				},
				{
					Value:    "License-Not-A-SPDX-Expression",
					Contents: "Non-SPDX license content", // content preserved
				},
			},
		},
		{
			name: "default license config should be LicenseContentExcludeAll",
			inputLicenses: []pkg.License{
				licenseWithSPDX,
				licenseWithoutSPDX,
			},
			cfg: cataloging.DefaultLicenseConfig(),
			expectedLicenses: []pkg.License{
				{
					SPDXExpression: "MIT",
				},
				{
					Value: "License-Not-A-SPDX-Expression",
				},
			},
		},
		{
			name: "invalid license content cataloging config results in the default case",
			inputLicenses: []pkg.License{
				licenseWithSPDX,
				licenseWithoutSPDX,
			},
			cfg: cataloging.LicenseConfig{
				IncludeContent: cataloging.LicenseContent("invalid"),
			},
			expectedLicenses: []pkg.License{
				{
					SPDXExpression: "MIT",
				},
				{
					Value:    "License-Not-A-SPDX-Expression",
					Contents: "", // content all removed
				},
			},
		},
		{
			name:          "Empty licenses",
			inputLicenses: []pkg.License{},
			cfg: cataloging.LicenseConfig{
				IncludeContent: cataloging.LicenseContentIncludeAll,
			},
			expectedLicenses: []pkg.License{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputPkg := &pkg.Package{
				Licenses: pkg.NewLicenseSet(tt.inputLicenses...),
			}

			inputPkg.SetID()
			originalID := inputPkg.ID()

			applyLicenseContentRules(inputPkg, tt.cfg)

			assert.Equal(t, originalID, inputPkg.ID(), "package ID changed unexpectedly")

			actualLicenses := inputPkg.Licenses.ToSlice()
			expectedLicenses := pkg.NewLicenseSet(tt.expectedLicenses...).ToSlice()

			assert.Equal(t, expectedLicenses, actualLicenses, "license contents do not match expected values")

		})
	}
}

func TestApplyComplianceRules_DropAndStub(t *testing.T) {
	p := pkg.Package{Name: "", Version: ""}
	p.SetID()
	ogID := p.ID()

	cfg := cataloging.ComplianceConfig{
		MissingName:    cataloging.ComplianceActionDrop,
		MissingVersion: cataloging.ComplianceActionStub,
	}

	isCompliant, replacement := applyComplianceRules(&p, cfg)
	require.NotNil(t, replacement)
	assert.Equal(t, packageReplacement{
		original: ogID,
		pkg:      p,
	}, *replacement)

	// the package should be dropped due to missing name (drop action) and its version should be stubbed
	assert.False(t, isCompliant)
	assert.Equal(t, cataloging.UnknownStubValue, p.Version)
}
