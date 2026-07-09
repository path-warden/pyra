package codehealth

// DefaultDetectors returns the full biomarker roster registered into the engine.
// Detector groups are appended by their respective files' register* functions as
// they are implemented.
func DefaultDetectors() []Detector {
	var d []Detector
	d = append(d, structuralDetectors()...)
	d = append(d, duplicationDetectors()...)
	d = append(d, organizationalDetectors()...)
	d = append(d, coverageDetectors()...)
	d = append(d, governanceDetectors()...)
	return d
}
