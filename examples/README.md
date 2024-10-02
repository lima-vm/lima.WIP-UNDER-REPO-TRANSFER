Run `limactl start template://fedora` to create a Lima instance named "fedora".

To open a shell, run `limactl shell fedora bash` or `LIMA_INSTANCE=fedora lima bash`.

- - -

⭐ = ["Tier 1"](#tier)

☆ = ["Tier 2"](#tier)

Default: [`default`](./default.yaml) (⭐Ubuntu, with containerd/nerdctl)

Distro:
- [`almalinux-8`](./almalinux-8.yaml): AlmaLinux 8
- [`almalinux-9`](./almalinux-9.yaml), `almalinux.yaml`: AlmaLinux 9
- [`alpine`](./alpine.yaml): ☆Alpine Linux
- [`archlinux`](./archlinux.yaml): ⭐Arch Linux
- [`centos-stream-9`](./centos-stream-9.yaml), `centos-stream.yaml`: CentOS Stream 9
- [`debian-11`](./debian-11.yaml): Debian GNU/Linux 11(bullseye)
- [`debian-12`](./debian-12.yaml), `debian.yaml`: ⭐Debian GNU/Linux 12(bookworm)
- [`fedora`](./fedora.yaml): ⭐Fedora
- [`opensuse-leap`](./opensuse-leap.yaml), `opensuse.yaml`: ⭐openSUSE Leap
- [`oraclelinux-8`](./oraclelinux-8.yaml): Oracle Linux 8
- [`oraclelinux-9`](./oraclelinux-9.yaml), `oraclelinux.yaml`: Oracle Linux 9
- [`rocky-8`](./rocky-8.yaml): Rocky Linux 8
- [`rocky-9`](./rocky-9.yaml), `rocky.yaml`: Rocky Linux 9
- [`ubuntu`](./ubuntu.yaml): Ubuntu (same as `default.yaml` but without extra YAML lines)
- [`ubuntu-lts`](./ubuntu-lts.yaml): Ubuntu LTS (same as `ubuntu.yaml` but pinned to an LTS version)
- [`experimental/gentoo`](./experimental/gentoo.yaml): [experimental] Gentoo
- [`experimental/opensuse-tumbleweed`](./experimental/opensuse-tumbleweed.yaml): [experimental] openSUSE Tumbleweed

Provisioning:
- [`ansible`](./ansible.yaml): Ansible, using [playbook.yaml](./ansible/playbook.yaml)

Container engines:
- [`apptainer`](./apptainer.yaml): Apptainer
- [`apptainer-rootful`](./apptainer-rootful.yaml): Apptainer (rootful)
- [`docker`](./docker.yaml): ⭐Docker
- [`docker-rootful`](./docker-rootful.yaml): Docker (rootful)
- [`podman`](./podman.yaml): Podman
- [`podman-rootful`](./podman-rootful.yaml): Podman (rootful)
- LXD is installed in the default Ubuntu template, so there is no `lxd.yaml`

Container image builders:
- [`buildkit`](./buildkit.yaml): BuildKit

Container orchestration:
- [`faasd`](./faasd.yaml): [Faasd](https://docs.openfaas.com/deployment/faasd/)
- [`k3s`](./k3s.yaml): Kubernetes via k3s
- [`k8s`](./k8s.yaml): Kubernetes via kubeadm
- [`experimental/u7s`](./experimental/u7s.yaml): [Usernetes](https://github.com/rootless-containers/usernetes): Rootless Kubernetes

Optional feature enablers:
- [`vmnet`](./vmnet.yaml): ⭐enable [`vmnet.framework`](../docs/network.md)
- [`experimental/9p`](./experimental/9p.yaml): [experimental] use 9p mount type
- [`experimental/virtiofs-linux`](./experimental/9p.yaml): [experimental] use virtiofs mount type for Linux
- [`experimental/armv7l`](./experimental/armv7l.yaml): [experimental] ARMv7
- [`experimental/riscv64`](./experimental/riscv64.yaml): [experimental] RISC-V
- [`experimental/net-user-v2`](./experimental/net-user-v2.yaml): [experimental] user-v2 network
  to enable VM-to-VM communication without root privilege
- [`experimental/vnc`](./experimental/vnc.yaml): [experimental] use vnc display and xorg server
- [`experimental/alsa`](./experimental/alsa.yaml): [experimental] use alsa and default audio device

Lost+found:
- ~`centos`~: Removed in Lima v0.8.0, as CentOS 8 reached [EOL](https://www.centos.org/centos-linux-eol/).
  Replaced by [`almalinux`](./almalinux.yaml), [`centos-stream`](./centos-stream.yaml), [`oraclelinux`](./oraclelinux.yaml),
  and [`rocky`](./rocky.yaml).
- ~`singularity`~: Moved to [`apptainer-rootful`](./apptainer-rootful.yaml) in Lima v0.13.0, as Singularity was renamed to Apptainer.
- ~`experimental/apptainer`~: Moved to [`apptainer`](./apptainer.yaml) in Lima v0.13.0.
- ~`experimental/{almalinux,centos-stream-9,oraclelinux,rocky}-9`~: Moved to [`almalinux-9`](./almalinux-9.yaml), [`centos-stream-9`](./centos-stream-9.yaml),
  [`oraclelinux-9`](./oraclelinux-9.yaml), and [`rocky-9`](./rocky-9.yaml) in Lima v0.13.0.
- ~`nomad`~: Removed in Lima v0.17.1, as Nomad is [no longer free software](https://github.com/hashicorp/nomad/commit/b3e30b1dfa185d9437a25830522da47b91f78816)
- ~`centos-stream-8`~: Remove in Lima v0.23.0, as CentOS Stream 8 reached [EOL](https://blog.centos.org/2023/04/end-dates-are-coming-for-centos-stream-8-and-centos-linux-7/).
- ~`deprecated/centos-7`~: Remove in Lima v0.23.0, as CentOS 7 reached [EOL](https://blog.centos.org/2023/04/end-dates-are-coming-for-centos-stream-8-and-centos-linux-7/).

## Tier

- "Tier 1" (marked with ⭐): Good stability. Regularly tested on the CI.
- "Tier 2" (marked with ☆): Moderate stability. Regularly tested on the CI.

Other yamls are tested only occasionally and manually.
