# CI

Continuous integration for this project is currently split between Wikimedia Gitlab shared runners and custom wmcli runners.

The shared runners are used where possible.
The custom mwcli runners are used when docker in docker is needed (integration tests).

This means that the FULL CI will NOT work for forks of this project, only for actual project branches.

## Custom runners

Custom runners run on the `mwcli` wikimedia.cloud project (the legacy name of this project).

There are currently 2 runners:
 - gitlab-runner-addshore-1017.mwcli.eqiad1.wikimedia.cloud (attached to volume `mwcli-ci-0001`) *bullseye (deprecated 2023-06-08)*
 - gitlab-runner-addshore-1018.mwcli.eqiad1.wikimedia.cloud (attached to volume `mwcli-ci-0002`) *NEW 2026*

You can view the usage at https://grafana.wmcloud.org/d/0g9N-7pVz/cloud-vps-project-board?orgId=1&from=now-24h&to=now&timezone=utc&var-project=mwcli&var-instance=$__all

### Maintenance

If the runner starts running out of space...

```sh
sudo docker system prune -af
sudo docker volume prune
```

If this doesn't free up enough space the next step would be to nuke the registry container and volume and recreate it!

### Initial Setup

#### Make a machine

Make a VM... Such as...

Details:
- Project Name: `mwcli`
- Instance Name: `gitlab-runner-addshore-1018`
- Count: `1`
Source: `trixie`
Flavour: `g4.cores4.ram8.disk20`

And then `Launch Instance`...

#### Attach a volume

See https://wikitech.wikimedia.org/wiki/Help:Adding_Disk_Space_to_Cloud_VPS_instances

- Make a volume of 40GB for the instance (or use an existing one)
- Attach a volume in the horizon UI https://horizon.wikimedia.org/project/volumes/
- Run `sudo wmcs-prepare-cinder-volume` on the instance
  - Select `/var/lib/docker` as the mount point
  - Wait for the mount to be created

The output will likely be something like this:

```
$ sudo wmcs-prepare-cinder-volume
This tool will partition, format, and mount a block storage device.


Attached storage devices:

    sda:  (the primary volume containing /)
    sdb: formatted as ext4, can be mounted

The only block device device available to mount is sdb.  Selecting.

Where would you like to mount it? </srv> /var/lib/docker
Ready to prepare and mount sdb on /var/lib/docker. OK to continue? <Y|n>y
Mounting on /var/lib/docker...
Updating fstab with UUID=d3923482-98db-4470-a074-da294e149472 /var/lib/docker ext4 discard,nofail,x-systemd.device-timeout=2s 0 2
...
Done.
```

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

Grab a key from https://hub.docker.com/settings/security being sure to use `Public Repo Read-only` and no expiration date.

Perform a `docker login` with your username and they READ ONLY PUBLIC key you created.

Note: if you made a fresh token, maybe keep it around as if you start a fresh mirror, you'll need the password again!

#### Install gitlab runner

From https://docs.gitlab.com/runner/install/linux-repository/

```sh
curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" -o script.deb.sh
sudo bash script.deb.sh
rm script.deb.sh

sudo apt install gitlab-runner
```

#### Register the runner

Head to https://gitlab.wikimedia.org/repos/releng/cli/-/settings/ci_cd#js-runners-settings

When creating the runner:
- Tags: `mwcli`
- Do not check `Run untagged jobs"
- Enter the `description` that matches the instance name
- Select `Lock to current projects`
Click `Create runner`

Then add the token that is provided to the below command, and run it on the instance...

```sh
sudo gitlab-runner register -n \
  --url https://gitlab.wikimedia.org/ \
  --token glrt-xxxxxxxxxxxxxxxxxxxxxxxxxxx \
  --executor docker \
  --limit 2 \
  --name "gitlab-runner-addshore-1018-docker" \
  --docker-image "docker:26.1.1" \
  --docker-privileged \
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

Create an authenticated pull through cache / mirror (using docker)
You can use the same username and password/key you used earlier

```sh
sudo docker run -d -p 6000:5000 \
    -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
    -e REGISTRY_PROXY_USERNAME=<TODO-USERNAME> \
    -e REGISTRY_PROXY_PASSWORD=<TODO-PASSWORD/KEY> \
    --restart always \
    --name registry registry:2
```

If you ever want to remove it and add it again, see `sudo docker rm -f registry`

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
sudo tee -a /etc/gitlab-runner/config.toml > /dev/null <<EOF
    pull_policy = ["always", "if-not-present"]
    [[runners.docker.services]]
      name = "docker:26.1.1-dind"
      command = ["--registry-mirror", "http://$(hostname --ip-address):6000"]
EOF
```

And restart the gitlab runner service:

```sh
sudo systemctl restart gitlab-runner
```
