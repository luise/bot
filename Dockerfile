FROM golang:1.7-onbuild

COPY . /go/src/app, RUN go get -d -v, and RUN go install -v.
