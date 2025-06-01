FROM golang:1.24.3

#OCI LABELS

LABEL org.opencontainers.image.author="Gokul Vootla"
LABEL org.opencontainers.image.title="Helm Templater" 
LABEL org.opencontainers.image.version="1.0"

RUN apt-get update && apt-get install -y \
  curl \
  git \
  bash \
  tar \
  gzip \
  && curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash \
  && apt-get clean


WORKDIR /app

COPY . .  

RUN go mod download 
RUN go build -o server . 

CMD ["./server"]
