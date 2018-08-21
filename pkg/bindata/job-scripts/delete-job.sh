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

# delete job
echo "Adding job {{ .JobName }}"
curl -s -X POST '{{ .Api }}job/{{ .JobName }}/doDelete' -v --header "X-CSRF-Token: $crumbToken"