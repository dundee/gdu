FROM debian

RUN apt-get update && apt-get upgrade -y
RUN apt-get install -y build-essential git
RUN apt-get install -y golang-go dh-make-golang dh-golang