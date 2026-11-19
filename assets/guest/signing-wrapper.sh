#!/bin/sh
# Generated installation substitutes the one stable, profile-bound socket.
SSH_AUTH_SOCK='@SOCKET@'
export SSH_AUTH_SOCK
exec /usr/bin/ssh-keygen "$@"
