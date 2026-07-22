FROM alpine:latest AS builder
RUN apk add --no-cache git
RUN git clone --depth 1 https://github.com/Tripouille/libftTester.git /tmp/libftTester

FROM alpine:latest
LABEL org.ft_hackthon.testsuites="1"
COPY . /var/ft_hackthon/testsuites
COPY --from=builder /tmp/libftTester /var/ft_hackthon/testsuites/libft-tester/tester
