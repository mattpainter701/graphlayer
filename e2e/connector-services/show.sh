#!/bin/sh
set -eu
case "$*" in
  version) printf 'Cisco Adaptive Security Appliance Software Version 9.18(4)\nHardware: ASA5525\nGraphLayer hosted connector fixture\n' ;;
  running-config|startup-config) cat /srv/configs/enterprise.asa.txt ;;
  *) printf 'Unsupported fixture command\n' >&2; exit 1 ;;
esac
