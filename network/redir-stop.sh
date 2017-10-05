#!/bin/bash

RULE="SOCKS"

iptables -t nat -F $RULE

#iptables -t nat -D OUTPUT -p tcp -j $RULE
iptables -t nat -D OUTPUT -p tcp --dport 80 -j $RULE

iptables -t nat -X $RULE


