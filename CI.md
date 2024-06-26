# CI

Continuous integration for this project is currently split between Wikimedia Gitlab shared runners and custom mwcli runners.

The shared runners are used where possible.
The custom mwcli runners are used when docker in docker is needed (integration tests).

This means that the FULL CI will NOT work for forks of this project, only for actual project branches.

## Custom runners

There are currently 2 runners:
 - gitlab-runner-addshore-1016.mwcli.eqiad1.wikimedia.cloud
 - gitlab-runner-addshore-1017.mwcli.eqiad1.wikimedia.cloud

### Maintenance

If the runner starts running out of space...

```sh
sudo docker system prune -af
sudo docker volume prune
```

If this doesn't free up enough space the next step would be to nuke the registry container and volume and recreate it!

### Initial Setup

#### Make a machine

Make a VM, such as `gitlab-runner-addshore-1017.mwcli.eqiad1.wikimedia.cloud`

#### Attach a volume

See https://wikitech.wikimedia.org/wiki/Help:Adding_Disk_Space_to_Cloud_VPS_instances

- Make a volume of 40GB for the instance
- Attach a volume in the horizon UI
- Run `sudo wmcs-prepare-cinder-volume` on the instance
  - Select `/var/lib/docker` as the mount point
  - Wait for the mount to be created

#### Install docker

```sh
sudo apt-get update
sudo apt-get remove docker docker-engine docker.io containerd runc
sudo apt-get --yes install \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg \
    lsb-release

curl -fsSL https://download.docker.com/linux/debian/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
echo \
  "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/debian \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt-get update
sudo apt-get install --yes docker-ce docker-ce-cli containerd.io
```

#### Authenticate to docker hub

Grab a key from https://hub.docker.com/settings/security

Perform a `docker login` with your username and they READ ONLY PUBLIC key you created.

#### Install gitlab runner

```sh
curl -LJO "https://gitlab-runner-downloads.s3.amazonaws.com/latest/deb/gitlab-runner_amd64.deb"
sudo dpkg -i gitlab-runner_amd64.deb
rm gitlab-runner_amd64.deb
```

#### Register the runner

WARNING: Support for registration tokens and runner parameters in the 'register' command has been deprecated in GitLab Runner 15.6 and will be replaced with support for authentication tokens. For more information, see https://gitlab.com/gitlab-org/gitlab/-/issues/380872

```sh
sudo gitlab-runner register -n \
  --url https://gitlab.wikimedia.org/ \
  --registration-token XXXreleng-mwcli-tokenXXX \
  --executor docker \
  --limit 2 \
  --name "gitlab-runner-addshore-1017-docker" \
  --docker-image "docker:26.1.1" \
  --docker-privileged \
  --tag-list mwcli \
  --docker-volumes "/certs/client"
```

Check it is registered @ https://gitlab.wikimedia.org/repos/releng/cli/-/settings/ci_cd#js-runners-settings

#### Extra configuration

##### Configure "global" runner jobs

Allow 2 jobs at once globally on this runner and restart gitlab runner.
(Any more than this and things get slow, timeout, use too much storage, fail etc)

```sh
sudo sed -i 's/^concurrent =.*/concurrent = 2/' "/etc/gitlab-runner/config.toml"
sudo systemctl restart gitlab-runner
```

##### Register local pull through cache / mirror

Reading:
 - https://about.gitlab.com/blog/2020/10/30/mitigating-the-impact-of-docker-hub-pull-requests-limits/
 - https://docs.docker.com/registry/recipes/mirror/#run-a-registry-as-a-pull-through-cache

Create an authenticaed pull through cache / mirror (using docker)
You can use the same username and password/key you used earlier

```sh
sudo docker run -d -p 6000:5000 \
    -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
    -e REGISTRY_PROXY_USERNAME=<TODO-USERNAME> \
    -e REGISTRY_PROXY_PASSWORD=<TODO-PASSWORD/KEY> \
    --restart always \
    --name registry registry:2
```

Add the mirror (You might need to do this as root, not sudo...):

```sh
sudo mkdir /etc/docker
# NOTE: If sudo doesn't work for the file change you may need to sudo su, and then run the echo as root...
sudo echo "{\"registry-mirrors\": [\"http://"$(hostname --ip-address)":6000\"]}" > /etc/docker/daemon.json
sudo service docker restart
```

Check with that a mirror appears in the info...

```sh
sudo docker system info
```

Also add the mirror for dind in `/etc/gitlab-runner/config.toml` to each runner it is needed for
https://docs.gitlab.com/ee/ci/docker/using_docker_build.html#enable-registry-mirror-for-dockerdind-service

You can also tweak the pull_policy to fallback to "if-not-present".

```sh
  [[runners.docker]]
    pull_policy = ["always", "if-not-present"]
    [[runners.docker.services]]
      name = "docker:26.1.1-dind"
      command = ["--registry-mirror", "http://172.16.5.159:6000"]
```

And restart the gitlab runner service:

```sh
sudo systemctl restart gitlab-runner
```
