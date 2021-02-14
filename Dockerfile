FROM debian:testing

RUN apt-get update && apt-get upgrade -y
RUN apt-get install -y build-essential git
RUN apt-get install -y golang-go dh-make-golang dh-golang
RUN apt-get install -y 	golang-github-gdamore-tcell.v2-dev \
               golang-github-mattn-go-isatty-dev \
               golang-github-rivo-tview-dev \
               golang-github-spf13-cobra-dev \
               golang-github-fatih-color-dev \
               golang-github-stretchr-testify-dev

RUN groupadd -r --gid=1000 go && \
    useradd -r --uid=1000 -b /home/go -d /home/go -m -s /bin/bash -g go go

USER go