package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const cmus = `Package: cmus
Version: 2.8.0+git20180917-1
Architecture: amd64
Maintainer: Debian Multimedia Maintainers <debian-multimedia@lists.debian.org>
Installed-Size: 838
Depends: libao4 (>= 1.1.0), libasound2 (>= 1.0.16), libc6 (>= 2.15), libcddb2, libcdio-cdda2 (>= 10.2+0.94+2), libcdio18 (>= 2.0.0), libdiscid0 (>= 0.2.2), libfaad2 (>= 2.7), libflac8 (>= 1.3.0), libmad0 (>= 0.15.1b-3),libmodplug1 (>= 1:0.8.8.5), libmpcdec6 (>= 1:0.1~r435), libncursesw6 (>= 6), libopusfile0 (>= 0.5), libsystemd0 (>= 221), libtinfo6 (>= 6), libvorbisfile3 (>= 1.1.2), libwavpack1 (>= 4.40.0)
Recommends: cmus-plugin-ffmpeg
Suggests: libjack-jackd2-0 (>= 1.9.10+20150825) | libjack-0.125, libpulse0 (>= 0.99.1), libroar2, libsamplerate0 (>= 0.1.7)
Section: sound
Priority: optional
Homepage: https://cmus.github.io/
Description: lightweight ncurses audio player
 C* Music Player is a modular and very configurable ncurses-based audio player.
 It has some interesting features like configurable colorscheme, mp3 and ogg
 streaming, it can be controlled with an UNIX socket, filters, album/artists
 sorting and a vi-like configuration interface.
 .
 It currently supports different input formats:
  - Ogg Vorbis
  - MP3 (with libmad)
  - FLAC
  - Wav
  - Modules (with libmodplug)
  - Musepack
  - AAC
  - Windows Media Audio
`

func TestControl(t *testing.T) {
	c, err := NewControlFromString(cmus)
	assert.NoError(t, err, "should not error when parsing")
	assert.NotNil(t, c, "result should not be nil")

	assert.Equal(t, `lightweight ncurses audio player
C* Music Player is a modular and very configurable ncurses-based audio player.
It has some interesting features like configurable colorscheme, mp3 and ogg
streaming, it can be controlled with an UNIX socket, filters, album/artists
sorting and a vi-like configuration interface.

It currently supports different input formats:
 - Ogg Vorbis
 - MP3 (with libmad)
 - FLAC
 - Wav
 - Modules (with libmodplug)
 - Musepack
 - AAC
 - Windows Media Audio
`, c.Values["Description"], "description should be parsed correctly")

	assert.Equal(t, cmus, c.String(), "re-encoded should match original")

	c.Set("Test", "asd:sdf")
	assert.Equal(t, "asd:sdf", c.Values["Test"], "should correctly set values with colons")

	c.Set("Test", "asd:sdf\nasd:sdf")
	assert.Equal(t, "asd:sdf\nasd:sdf", c.Values["Test"], "should correctly set multi-line values with colons")

	seen := map[string]bool{}
	for _, key := range c.Order {
		assert.NotContains(t, seen, key, "should not have duplicate keys in order")
		seen[key] = true
	}

	assert.True(t, c.MoveToOrderStart("Package"))
	assert.Equal(t, []string{
		"Package", "Version", "Architecture", "Maintainer", "Installed-Size", "Depends", "Recommends", "Suggests", "Section", "Priority", "Homepage", "Description", "Test",
	}, c.Order, "order should not change when moving start value to start")

	assert.False(t, c.MoveToOrderStart("sdfsdf"))
	assert.Equal(t, []string{
		"Package", "Version", "Architecture", "Maintainer", "Installed-Size", "Depends", "Recommends", "Suggests", "Section", "Priority", "Homepage", "Description", "Test",
	}, c.Order, "order should not change when moving nonexistent value to start")

	assert.True(t, c.MoveToOrderStart("Test"))
	assert.Equal(t, []string{
		"Test", "Package", "Version", "Architecture", "Maintainer", "Installed-Size", "Depends", "Recommends", "Suggests", "Section", "Priority", "Homepage", "Description",
	}, c.Order, "order should be correct")
}
