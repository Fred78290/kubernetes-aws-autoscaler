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
#ARG BASEIMAGE=gcr.io/distroless/static:nonroot
ARG BASEIMAGE=alpine:3.19
FROM alpine:3.19 AS builder
ARG TARGETPLATFORM
ARG BUILDPLATFORM

COPY out .
RUN mv /$TARGETPLATFORM/aws-autoscaler /aws-autoscaler ; chmod uog+x /aws-autoscaler

FROM $BASEIMAGE
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG USER=default

ENV HOME /home/$USER

LABEL maintainer="Frederic Boltz <frederic.boltz@gmail.com>"

COPY --from=builder /aws-autoscaler /usr/local/bin/aws-autoscaler

# add new user
RUN apk update \
	&& apk add --no-cache sudo openssh \
	&& adduser -D $USER -u 65532 \
	&& echo "$USER ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/$USER \
	&& chmod 0440 /etc/sudoers.d/$USER

USER $USER

EXPOSE 5200

CMD ["/usr/local/bin/aws-autoscaler"]
