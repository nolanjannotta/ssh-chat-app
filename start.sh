#!/bin/bash


docker run -d -p 2227:2227 -v ~/.ssh/chat-app:/root/.ssh/ ssh-chat