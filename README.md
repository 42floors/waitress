# Waitress

## Dependencies

- [Go](http://golang.org/)

## Installation

- go get
- go build

Running `go build` will result in a binary named `waitress` being created. This
can then be moved to the `/usr/local` or the preferred install path. 

## Running

Usage of waitress:
  -binding="0.0.0.0": bind the server to the specified ip
  -config="": conf file (see config.yml.sample)
  -port="3000": run the server on the specified port

## Image processing

The format is specified in the query string as the `s` option. The syntax of `s`
is similar to the [geometry](http://www.imagemagick.org/script/command-line-processing.php#geometry)
syntax in ImageMagick.

### Syntax

    s ::= ( width | 'x' height | width 'x' height ) ( '#' | '^' | '!' )?

### Size
  `s=700`: Resize the image to have a width of 700 (maintains aspect ratio)
  `s=x700`: Resize the image to have a height of 700 (maintains aspect ratio)
  `s=700x700`: Resize the image to have a width and height of 700. This
     maintains the aspect ratio and pads image with a background color.

### Options
  `#`: Crop the image to fill the specified size (This need to be URL escaped)
  `^`: The size specified is a minimum. The resulting image may be bigger
  `!`: Do not preserve the aspect ratio.

### Examples

- http://example.com/ec64fc472479c2cddada6ad58e802d492b5936e3.jpg?s=700
- http://example.com/ec64fc472479c2cddada6ad58e802d492b5936e3.jpg?s=x700
- http://example.com/ec64fc472479c2cddada6ad58e802d492b5936e3.jpg?s=700x700
- http://example.com/ec64fc472479c2cddada6ad58e802d492b5936e3.jpg?s=700^
- http://example.com/ec64fc472479c2cddada6ad58e802d492b5936e3.jpg?s=700x700^
- http://example.com/ec64fc472479c2cddada6ad58e802d492b5936e3.jpg?s=700x467%23
- http://example.com/ec64fc472479c2cddada6ad58e802d492b5936e3.jpg?s=700x467!

## TODO: Makefile