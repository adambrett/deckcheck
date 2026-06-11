package dataset

// The blank imports register a decoder with image.Decode for every
// extension in imageExtensions, so the supported-format policy and the
// linked-in decoding capability live in one file and cannot drift.
import (
	_ "image/gif"  // register GIF decoder with the image package
	_ "image/jpeg" // register JPEG decoder with the image package
	_ "image/png"  // register PNG decoder with the image package
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"  // register BMP decoder with the image package
	_ "golang.org/x/image/webp" // register WebP decoder with the image package
)

// imageExtensions is the set of file extensions that DeckCheck
// recognises as previewable images.
var imageExtensions = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".gif":  {},
	".bmp":  {},
	".webp": {},
}

// IsImageFile reports whether name has a supported image extension. The check
// is case-insensitive so JPEG, PNG, WebP, etc. all count.
func IsImageFile(name string) bool {
	_, ok := imageExtensions[strings.ToLower(filepath.Ext(name))]
	return ok
}
