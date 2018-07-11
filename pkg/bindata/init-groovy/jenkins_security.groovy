#!groovy

import jenkins.model.*
import hudson.security.*
import jenkins.model.JenkinsLocationConfiguration
import hudson.security.csrf.DefaultCrumbIssuer

def instance = Jenkins.getInstance()

println "--> creating local user '{{.User}}'"

def hudsonRealm = new HudsonPrivateSecurityRealm(false)
hudsonRealm.createAccount('{{.User}}', '{{.Password}}')
instance.setSecurityRealm(hudsonRealm)

def strategy = new FullControlOnceLoggedInAuthorizationStrategy()
strategy.setAllowAnonymousRead(false)
instance.setAuthorizationStrategy(strategy)
instance.save()

println "--> setting jenkins url '{{.Url}}'"

jenkinsLocation = JenkinsLocationConfiguration.get()
jenkinsLocation.setUrl('{{.Url}}')
jenkinsLocation.setAdminAddress('{{.AdminEmail}}')
jenkinsLocation.save()

println "--> setting up default crumbs issuer"
Jenkins.instance.setCrumbIssuer(new DefaultCrumbIssuer(true))
Jenkins.instance.save()

println "--> setting up JNLP"
Set<String> agentProtocolsList = ['JNLP4-connect', 'Ping']
Jenkins.instance.setAgentProtocols(agentProtocolsList)
Jenkins.instance.setSlaveAgentPort({{ .AgentPort }})

Jenkins.instance.setNumExecutors({{ .Executors }})
Jenkins.instance.save()
