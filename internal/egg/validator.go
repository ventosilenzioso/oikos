package egg

import "errors"

func Validate(e Egg) error {
	if e.Name == "" {
		return errors.New("egg.name wajib diisi")
	}
	if e.Slug == "" {
		return errors.New("egg.slug wajib diisi")
	}
	if e.Startup.Command == "" {
		return errors.New("egg.startup.command wajib diisi")
	}
	if e.Build.Dockerfile == "" {
		return errors.New("egg.build.dockerfile wajib diisi")
	}
	return nil
}
