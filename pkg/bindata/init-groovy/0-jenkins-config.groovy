#!groovy

import jenkins.model.*
import hudson.security.*
import jenkins.model.JenkinsLocationConfiguration
import hudson.security.csrf.DefaultCrumbIssuer
import jenkins.security.s2m.*
import org.jenkinsci.plugins.authorizeproject.*
import org.jenkinsci.plugins.authorizeproject.strategy.*
import jenkins.security.QueueItemAuthenticatorConfiguration
import javaposse.jobdsl.dsl.DslScriptLoader
import javaposse.jobdsl.plugin.JenkinsJobManagement

def instance = Jenkins.getInstance()
println "--> creating local user '{{.User}}'"

def hudsonRealm = new HudsonPrivateSecurityRealm(false)
hudsonRealm.createAccount('{{.User}}', '{{.Password}}')
instance.setSecurityRealm(hudsonRealm)

def strategy = new FullControlOnceLoggedInAuthorizationStrategy()
strategy.setAllowAnonymousRead(false)
instance.setAuthorizationStrategy(strategy)

println "--> setting jenkins url '{{.Url}}'"

jenkinsLocation = JenkinsLocationConfiguration.get()
jenkinsLocation.setUrl('{{.Url}}')
jenkinsLocation.setAdminAddress('{{.AdminEmail}}')
jenkinsLocation.save()

println "--> setting up default crumbs issuer"
instance.setCrumbIssuer(new DefaultCrumbIssuer(true))

println "--> setting up JNLP"
Set<String> agentProtocolsList = ['JNLP4-connect', 'Ping']
instance.setAgentProtocols(agentProtocolsList)
instance.setSlaveAgentPort({{ .AgentPort }})
instance.setNumExecutors({{ .Executors }})

println "--> enable Agent to master security subsystem"
instance.injector.getInstance(AdminWhitelistRule.class).setMasterKillSwitch(false);

println "--> setting up project authorization"
QueueItemAuthenticatorConfiguration.get().getAuthenticators().add(new GlobalQueueItemAuthenticator(new TriggeringUsersAuthorizationStrategy()))

println "--> setting up seed job"
new DslScriptLoader(new JenkinsJobManagement(System.out, [:], new File('.'))).runScript(new File('/var/jenkins_home/init.groovy.d/seed-job-dsl').text)

println "--> saving instance"
instance.save()



