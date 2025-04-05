#!/bin/bash




sudo docker build -t ssh-chat .

containerId = $(sudo docker ps --filter "ancestor=ssh-chat" --format "{{.ID}}")
sudo docker stop $containerId

sudo docker run -d -p 2227:2227 -v ~/.ssh/chat-app:/root/.ssh/ ssh-chat
sudo docker system prune -af