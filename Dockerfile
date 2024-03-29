FROM golang:alpine as builder
RUN mkdir /build 
ADD . /build/
WORKDIR /build 
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o main .

FROM gcr.io/distroless/base
COPY --from=builder /build/main /gomirror/
WORKDIR /gomirror
ENV FILE ""
CMD ["./main"]