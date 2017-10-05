#!/bin/bash

RULE="SOCKS"
SOCKIP="1.2.3..4"
SOCKPORT="1080"

#======= clean ==========
iptables -t nat -F $RULE

#iptables -t nat -D PREROUTING -p tcp -j $RULE
#iptables -t nat -D OUTPUT -p tcp -j $RULE
iptables -t nat -D OUTPUT -p tcp --dport 80 -j $RULE

iptables -t nat -X $RULE


#======= start ==========

iptables -t nat -N $RULE

iptables -t nat -A $RULE -p tcp -d $SOCKIP --dport $SOCKPORT -j RETURN # ip & port
#iptables -t nat -A $RULE -p tcp --dport $SOCKPORT -j RETURN # port only
#iptables -t nat -A $RULE -d $SOCKIP -j RETURN # ip only


#iptables -t nat -A $RULE -d 0.0.0.0/8 -j RETURN
iptables -t nat -A $RULE -d 10.0.0.0/8 -j RETURN
iptables -t nat -A $RULE -d 127.0.0.0/8 -j RETURN
iptables -t nat -A $RULE -d 192.168.0.0/16 -j RETURN
#iptables -t nat -A $RULE -p tcp -j REDIRECT --to-ports 7777
iptables -t nat -A $RULE -p tcp -s 192.168.0.0/16 -j REDIRECT --to-ports 7777

#iptables -t nat -I PREROUTING -p tcp -j $RULE
#iptables -t nat -A OUTPUT -p tcp -j $RULE # all tcp

iptables -t nat -A OUTPUT -p tcp --dport 80 -j $RULE # only port


