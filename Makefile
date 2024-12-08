deploy:
	ssh isu14-1 " \
		cd /home/isucon; \
		git checkout .; \
		git fetch; \
		git checkout $(BRANCH); \
		git reset --hard origin/$(BRANCH)"

build:
	ssh isu14-1 " \
		cd /home/isucon/webapp/go; \
		/home/isucon/local/golang/bin/go build -o isuride"

go-deploy:
	scp ./webapp/go/isuride isu14-1:/home/isucon/webapp/go/

go-deploy-dir:
	scp -r ./webapp/go isu14-1:/home/isucon/webapp/

restart:
	ssh isu14-1 "sudo systemctl restart isuride-go.service"

mysql-deploy:
	ssh isu14-1 "sudo dd of=/etc/mysql/mysql.conf.d/mysqld.cnf" < ./etc/mysql/mysql.conf.d/mysqld.cnf

mysql-rotate:
	ssh isu14-1 "sudo rm -f /var/log/mysql/mysql-slow.log"

mysql-restart:
	ssh isu14-1 "sudo systemctl restart mysql.service"

nginx-deploy:
	ssh isu14-1 "sudo dd of=/etc/nginx/nginx.conf" < ./etc/nginx/nginx.conf
	ssh isu14-1 "sudo dd of=/etc/nginx/sites-enabled/isuride.conf" < ./etc/nginx/sites-enabled/isuride.conf

nginx-rotate:
	ssh isu14-1 "sudo rm -f /var/log/nginx/access.log"

nginx-reload:
	ssh isu14-1 "sudo systemctl reload nginx.service"

nginx-restart:
	ssh isu14-1 "sudo systemctl restart nginx.service"

env-deploy:
	ssh isu14-1 "sudo dd of=/home/isucon/env.sh" < ./env.sh

.PHONY: bench
bench:
	ssh isucon13-bench " \
		cd /home/isucon/bench; \
		./bench -target-addr 172.31.41.209:443"

journalctl:
	ssh isu14-1 "sudo journalctl -xef"

nginx-log:
	ssh isu14-1 "sudo tail -f /var/log/nginx/access.log"

pt-query-digest:
	ssh isu14-1 "sudo pt-query-digest --limit 10 /var/log/mysql/mysql-slow.log"

ALPSORT=sum
# ^/api/app/nearby-chairs\?
# ^/api/app/rides/[0-9A-Za-z]+/evaluation
# ^/api/chair/rides/[0-9A-Za-z]+/status
# ^/api/owner/sales\?
ALPM=^/api/chair/rides/[0-9A-Za-z]+/status,^/api/app/rides/[0-9A-Za-z]+/evaluation,^/api/app/nearby-chairs\?,^/api/owner/sales\?
OUTFORMAT=count,method,uri,min,max,sum,avg,p99

alp:
	ssh isu14-1 "sudo alp ltsv --file=/var/log/nginx/access.log --nosave-pos --pos /tmp/alp.pos --sort $(ALPSORT) --reverse -o $(OUTFORMAT) -m $(ALPM) -q"

.PHONY: pprof
pprof:
	ssh isu14-1 " \
		/home/isucon/local/golang/bin/go tool pprof -seconds=120 /home/isucon/webapp/go/isuride http://0.0.0.0:6060/debug/pprof/profile"

pprof-show:
	$(eval latest := $(shell ssh isu14-1 "ls -rt ~/pprof/ | tail -n 1"))
	scp isu14-1:~/pprof/$(latest) ./pprof
	go tool pprof -http=":1080" ./pprof/$(latest)

pprof-kill:
	ssh isu14-1 "pgrep -f 'pprof' | xargs kill;"
