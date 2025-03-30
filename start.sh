#!/bin/bash


sudo docker run -d -p 2227:2227 -v ~/.ssh/chat-app:/root/.ssh/ ssh-chat
sudo docker system prune -af
