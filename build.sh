#!/bin/bash

export CGO_CPPFLAGS="-I/opt/homebrew/Cellar/leptonica/1.86.0/include -I/opt/homebrew/Cellar/tesseract/5.5.1/include"
export CGO_LDFLAGS="-L/opt/homebrew/Cellar/tesseract/5.5.1/lib -L/opt/homebrew/Cellar/leptonica/1.86.0/lib"

go build -o crop-ocr crop.go
