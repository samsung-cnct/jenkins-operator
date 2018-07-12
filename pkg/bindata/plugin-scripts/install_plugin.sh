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

# assumes cliAuth mounted to /jenkins/auth/cliAuth
echo "Installing plugin {{ .PluginId }}@{{ .PluginVersion }}"

curl -X POST -d '<jenkins><install plugin="{{ .PluginId }}@{{ .PluginVersion }}" /></jenkins>' --header 'Content-Type: text/xml' --header "X-CSRF-Token: $crumbToken" {{ .Api }}pluginManager/installNecessaryPlugins

curl -X POST --header "X-CSRF-Token: $crumbToken" {{ .Api }}/safeRestart

# write plugin config to file
cat >/tmp/{{ .PluginId }}.{{ .PluginVersion }}.config <<EOL
{{ .PluginConfig }}
EOL

echo "Wrote to /tmp/{{ .PluginId }}.{{ .PluginVersion }}.config:"
cat /tmp/{{ .PluginId }}.{{ .PluginVersion }}.config

if [ -s /tmp/{{ .PluginId }}.{{ .PluginVersion }}.config ]; then

    # Wait for Jenkins to load
    while ! curl -s --head --request GET {{ .Api }} | grep "200 OK" > /dev/null;
    do
        echo "CLI: Waiting for jenkins.."
        sleep 3
    done

    # run plugin config
    echo "Running configuration for plugin {{ .PluginId }}@{{ .PluginVersion }}"

    curl -d "script=$(cat /tmp/{{ .PluginId }}.{{ .PluginVersion }}.config)" -v --header "X-CSRF-Token: $crumbToken" {{ .Api }}scriptText
fi