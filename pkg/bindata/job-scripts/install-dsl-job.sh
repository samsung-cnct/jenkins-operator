#!/usr/bin/env bash
printenv

# Wait for Jenkins to load
while ! curl -s --head --request GET {{ .Api }} | grep "200 OK" > /dev/null;
do
	echo "CLI: Waiting for jenkins.."
	sleep 3
done

# get CSRF protection token
echo "Getting CSRF protection token"
crumbToken=$(curl '{{ .Api }}crumbIssuer/api/xml?xpath=concat(//crumbRequestField,":",//crumb)')

# write job dsl to a file
cat >/tmp/{{ .JobName }}.dsl <<EOL
{{ .JobDsl }}
EOL

echo "Wrote to /tmp/{{ .JobName }}.dsl:"
cat /tmp/{{ .JobName }}.dsl

# start seed job with parameters
echo "Starting {{ .Api }}job/create-jenkins-jobs/build"
curl -X POST {{ .Api }}job/create-jenkins-jobs/build --form file0=@/tmp/{{ .JobName }}.dsl --form json='{"parameter": [{"name":"job.dsl", "file":"file0"}]}' -v --header "X-CSRF-Token: $crumbToken"
