#!/usr/bin/env bash

# Wait for Jenkins to load
while ! curl -s --head --request GET {{ .Api }} | grep "200 OK" > /dev/null;
do
	echo "CLI: Waiting for jenkins.."
	sleep 3
done

# get CSRF protection token
echo "Getting CSRF protection token"
crumbToken=$(curl '{{ .Api }}crumbIssuer/api/xml?xpath=concat(//crumbRequestField,":",//crumb)')

# assumes cliAuth mounted to /jenkins/auth/cliAuth
echo "Installing plugin {{ .PluginId }}@{{ .PluginVersion }}"

curl -X POST -d '<jenkins><install plugin="{{ .PluginId }}@{{ .PluginVersion }}" /></jenkins>' --header 'Content-Type: text/xml' --header "X-CSRF-Token: $crumbToken" {{ .Api }}pluginManager/installNecessaryPlugins

curl -X POST --header "X-CSRF-Token: $crumbToken" {{ .Api }}/safeRestart