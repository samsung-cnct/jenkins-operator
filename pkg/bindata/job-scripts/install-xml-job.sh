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

# write job xml to a file
cat >/tmp/{{ .JobName }}.xml <<EOL
{{ .JobXml }}
EOL

echo "Wrote to /tmp/{{ .JobName }}.xml:"
cat /tmp/{{ .JobName }}.xml

# create job
echo "Adding job {{ .JobName }}"
curl -s -X POST '{{ .Api }}createItem?name={{ .JobName }}' --data-binary @/tmp/{{ .JobName }}.xml -v --header "X-CSRF-Token: $crumbToken" --header "Content-Type:text/xml"