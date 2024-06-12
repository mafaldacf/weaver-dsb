FROM python:slim

# ----------------
# install packages
# ----------------
RUN set -ex ;\
    apt-get update ;\
    apt-get install -y --no-install-recommends \
        # gcp
        apt-transport-https \
        ca-certificates \
        openssl \
        gnupg \
        curl \
        wget \
        jq \
        lsb-release \
        nano \
        ;\
    rm -rf /var/lib/apt/lists/*

RUN mkdir -p /socialnetwork
COPY requirements.txt .
COPY gcp/credentials.json .

# -------------------
# python requirements
# -------------------
RUN set -ex ;\
    pip install --no-cache-dir --upgrade -r requirements.txt ;\
    rm requirements.txt
    
# ----
# wrk2
# ----
RUN set -ex ;\
    apt-get update ;\
    apt-get install -y libssl-dev luarocks ;\
    luarocks install lua-json ;\
    luarocks install luasocket ;\
    luarocks install penlight

# ------
# docker
# ------
# add docker official gpg key
RUN set -ex ;\
    apt-get update ;\
    curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
# add repository to apt sources
RUN set -ex ;\
    echo \
        "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian \
        $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
        tee /etc/apt/sources.list.d/docker.list > /dev/null
# install docker latest version with docker compose
RUN set -ex ;\
    apt-get update ;\
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# ---------
# terraform
# ---------
# install hashicorp gpg key
RUN set -ex ;\
    wget -O- https://apt.releases.hashicorp.com/gpg | \
    gpg --dearmor | \
    tee /usr/share/keyrings/hashicorp-archive-keyring.gpg > /dev/null
# verify key fingerprint
RUN set -ex ;\
    gpg --no-default-keyring \
    --keyring /usr/share/keyrings/hashicorp-archive-keyring.gpg \
    --fingerprint
# add official hashicorp repository to system
RUN set -ex ;\
    echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] \
    https://apt.releases.hashicorp.com $(lsb_release -cs) main" | \
    tee /etc/apt/sources.list.d/hashicorp.list
# update package information from hashicorp
RUN set -ex ;\
    apt-get update
# install terraform from new repository
RUN set -ex ;\
    apt-get install -y terraform

# -------
# ansible
# -------
RUN set -ex ;\
    apt-get update ;\  
    apt-get upgrade ;\  
    apt-get install -y software-properties-common ;\  
    python -m pip install ansible

# ------------
# google cloud
# ------------
RUN set -ex ;\
    echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list ;\
    curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key --keyring /usr/share/keyrings/cloud.google.gpg add - ;\
    apt-get update ;\
    apt-get install -y --no-install-recommends \
        google-cloud-sdk \
        ;\
    rm -rf /var/lib/apt/lists/*
ENV GOOGLE_APPLICATION_CREDENTIALS credentials.json
RUN set -ex ;\
    export GCP_PROJECT_ID=$(jq -r '.project_id' ${GOOGLE_APPLICATION_CREDENTIALS}) ;\
    gcloud config configurations list ;\
    gcloud config configurations activate default ;\
    gcloud auth activate-service-account --key-file=${GOOGLE_APPLICATION_CREDENTIALS} ;\
    gcloud config set project $GCP_PROJECT_ID ;\
    gcloud auth configure-docker ;\
    gcloud config list ;\
    rm credentials.json

CMD bash
