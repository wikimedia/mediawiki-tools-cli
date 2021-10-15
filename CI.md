# CI

Continuous integration for this project is currently set up on a dedicated Cloud VPM machine.

Currently this CI will NOT work for forks of this project, only for actual project branches.

## Maintenance

If the runner starts running out of space...

```sh
sudo docker system prune --force
sudo docker volume prune
```

If this doesn't free up enough space the next step would be to nuke the registry container and volume and recreate it!

## Initial Setup

### Make a machine

Make a VM, such as `gitlab-runner-addshore-1004.integration.eqiad1.wikimedia.cloud`

### Install docker

```sh
sudo apt-get update
sudo apt-get remove docker docker-engine docker.io containerd runc
sudo apt-get install \
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
sudo apt-get install docker-ce docker-ce-cli containerd.io
```

### Install gitlab runner

```sh
curl -LJO "https://gitlab-runner-downloads.s3.amazonaws.com/latest/deb/gitlab-runner_amd64.deb"
sudo dpkg -i gitlab-runner_amd64.deb
rm gitlab-runner_amd64.deb
```

### Register the runner

```sh
sudo gitlab-runner register -n \
  --url https://gitlab.wikimedia.org/ \
  --registration-token XXXreleng-mwcli-tokenXXX \
  --executor docker \
  --limit 4 \
  --name "gitlab-runner-addshore-1004-docker-01" \
  --docker-image "docker:19.03.15" \
  --docker-privileged \
  --docker-volumes "/certs/client"
```

### Extra configuration

#### Configure "global" runner jobs

Allow 4 jobs at once globally on this runner and restart gitlab runner

```sh
sudo sed -i 's/^concurrent =.*/concurrent = 4/' "/etc/gitlab-runner/config.toml"
sudo systemctl restart gitlab-runner
```

#### Register custom local docker mirror

Mainly from https://about.gitlab.com/blog/2020/10/30/mitigating-the-impact-of-docker-hub-pull-requests-limits/

Create a mirror (using docker):

```sh
sudo docker run -d -p 6000:5000 \
    -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
    --restart always \
    --name registry registry:2
```

Get the IP address:

```sh
hostname --ip-address
```

Add the mirror (You might need to do this as root, not sudo...):

```sh
sudo echo '{"registry-mirrors": ["http://<CUSTOM IP>:<PORT>"]}' > /etc/docker/daemon.json
sudo service docker restart
```

Check with:

```sh
sudo docker system info
```

Also add the mirror for dind in `/etc/gitlab-runner/config.toml` to each runner it is needed for
https://docs.gitlab.com/ee/ci/docker/using_docker_build.html#enable-registry-mirror-for-dockerdind-service

```sh
    [[runners.docker.services]]
      name = "docker:19.03.15-dind"
      command = ["--registry-mirror", "http://<CUSTOM IP>:<PORT>"]
```

And restart the gitlab runner service:

```sh
sudo systemctl restart gitlab-runner
```