# Copyright 2019 Frederic Boltz Author. All rights reserved
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#ARG BASEIMAGE=gcr.io/distroless/static:latest-amd64
FROM alpine AS builder
ARG TARGETPLATFORM
ARG BUILDPLATFORM

RUN apk add -U --no-cache ca-certificates

COPY out .

RUN mv /$TARGETPLATFORM/aws-autoscaler /aws-autoscaler

FROM ubuntu:20.10

LABEL maintainer="Frederic Boltz <frederic.boltz@gmail.com>"

COPY --from=builder /etc/ssl /etc/ssl
COPY --from=builder /aws-autoscaler /usr/local/bin/aws-autoscaler
RUN chmod uog+x /usr/local/bin/aws-autoscaler

EXPOSE 5200

CMD ["/usr/local/bin/aws-autoscaler"]
