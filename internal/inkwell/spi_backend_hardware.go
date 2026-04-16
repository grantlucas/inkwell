//go:build hardware

package inkwell

func init() {
	createSPIBackendFn = func(_ *Config, _ *DisplayProfile) (Hardware, error) {
		return NewSPIHardware()
	}
}
