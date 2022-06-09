# jiratools
Handy tool to query Jira API for stuff

### Installation

```
git clone git@github.com:mfojtik/jiratools
cd jiratools/
go build
```

This will give you `jiratools` binary.

### Configuration

Go to Jira -> Profile/[Personal Access Token](https://issues.redhat.com/secure/ViewProfile.jspa?selectedTab=com.atlassian.pats.pats-plugin:jira-user-personal-access-tokens) and make a token.
Then export your token in bash as `JIRA_TOKEN`. 

```
Usage of ./jiratools:
      --blocker strings          comma separated list of valid blocker bug flag value (eg. '+,-,?'), default is all bugs
      --blockers-only            if set, the list will include bugs without blocker bug flag set (default true)
      --counts                   show counts
      --not-status strings       only include bugs that has not this status
      --status strings           only include bugs that has this status
      --target-version strings   comma separated list of target version (fixed in) values (eg. '---,4.11.z'), default is all versions
      --teams strings            only show bugs for components owned by given team name (valid teams: "api"), default is all teams
      --versions strings         comma separated list of version (affected) values (eg. '4.10,4.11'), default is all versions
```

