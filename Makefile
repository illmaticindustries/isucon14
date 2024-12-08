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
	ssh isu11q-1 "sudo journalctl -xef"

nginx-log:
	ssh isu11q-1 "sudo tail -f /var/log/nginx/access.log"

pt-query-digest:
	ssh isu14-1 "sudo pt-query-digest --limit 10 /var/log/mysql/mysql-slow.log"

ALPSORT=sum
# /api/user/mikakosasaki0/icon
# /api/user/jtakahashi0/theme
# /api/user/iyamamoto1/livestream
# /api/user/suzukitsubasa0/statistics
# /api/livestream/search?tag=%E6%98%A0%E7%94%BB%E3%83%AC%E3%83%93%E3%83%A5%E3%83%BC
# /api/livestream/7497
# /api/livestream/7508/report
# /api/livestream/7515/ngwords
# /api/livestream/7508/reaction
# /api/livestream/7510/livecomment/1004/report
# /api/livestream/7497/exit
# /api/livestream/7497/enter
# /api/livestream/7510/livecomment
# /api/livestream/7581/statistics
ALPM=/api/livestream/[0-9]+/report,/api/user/[0-9a-zA-Z]+/icon,/api/user/[0-9a-zA-Z]+/livestream,/api/livestream/[0-9]+/ngwords,/api/livestream/[0-9]+/reaction,/api/livestream/[0-9]+/livecomment/[0-9]+/report,/api/livestream/[0-9]+/exit,/api/livestream/[0-9]+/enter,/api/user/[0-9a-zA-Z]+/theme,/api/livestream/[0-9]+/livecomment,/api/livestream/[0-9]+/moderate,/api/user/[0-9a-zA-Z]+/statistics,/api/livestream/search,/api/livestream/[0-9]+/statistics
OUTFORMAT=count,method,uri,min,max,sum,avg,p99

alp:
	ssh isu14-1 "sudo alp ltsv --file=/var/log/nginx/access.log --nosave-pos --pos /tmp/alp.pos --sort $(ALPSORT) --reverse -o $(OUTFORMAT) -m $(ALPM) -q"

.PHONY: pprof
pprof:
	ssh isu14-1 " \
		/home/isucon/local/golang/bin/go tool pprof -seconds=120 /home/isucon/webapp/go/isuride http://localhost:6060/debug/pprof/profile"

pprof-show:
	$(eval latest := $(shell ssh isu14-1 "ls -rt ~/pprof/ | tail -n 1"))
	scp isu14-1:~/pprof/$(latest) ./pprof
	go tool pprof -http=":1080" ./pprof/$(latest)

pprof-kill:
	ssh isu14-1 "pgrep -f 'pprof' | xargs kill;"
