FROM golang:1.25-bookworm

ARG TZ=America/Sao_Paulo
ENV TZ="$TZ"

ARG CLAUDE_CODE_VERSION=latest

# Install system dependencies and firewall tools
RUN apt-get update && apt-get install -y --no-install-recommends \
  less \
  git \
  procps \
  sudo \
  fzf \
  zsh \
  man-db \
  unzip \
  gnupg2 \
  iptables \
  ipset \
  iproute2 \
  dnsutils \
  jq \
  nano \
  vim \
  curl \
  wget \
  ca-certificates \
  make \
  && apt-get clean && rm -rf /var/lib/apt/lists/*

# Install Node.js 20 (required for Claude Code)
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
  apt-get install -y --no-install-recommends nodejs && \
  apt-get clean && rm -rf /var/lib/apt/lists/*

# Install Go development tools into /usr/local/bin (available to all users)
RUN GOTOOLCHAIN=auto GOBIN=/usr/local/bin go install github.com/air-verse/air@latest && \
  GOTOOLCHAIN=auto GOBIN=/usr/local/bin go install github.com/pressly/goose/v3/cmd/goose@latest && \
  GOTOOLCHAIN=auto GOBIN=/usr/local/bin go install github.com/evilmartians/lefthook@latest && \
  GOTOOLCHAIN=auto GOBIN=/usr/local/bin go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && \
  GOTOOLCHAIN=auto GOBIN=/usr/local/bin go install github.com/swaggo/swag/cmd/swag@latest && \
  GOTOOLCHAIN=auto GOBIN=/usr/local/bin go install golang.org/x/tools/gopls@latest && \
  GOTOOLCHAIN=auto GOBIN=/usr/local/bin go install golang.org/x/tools/cmd/goimports@latest

# Install Claude Code
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}

# Install git-delta for enhanced diffs
ARG GIT_DELTA_VERSION=0.18.2
RUN ARCH=$(dpkg --print-architecture) && \
  wget -q "https://github.com/dandavison/delta/releases/download/${GIT_DELTA_VERSION}/git-delta_${GIT_DELTA_VERSION}_${ARCH}.deb" && \
  dpkg -i "git-delta_${GIT_DELTA_VERSION}_${ARCH}.deb" && \
  rm "git-delta_${GIT_DELTA_VERSION}_${ARCH}.deb"

# Create non-root user
ARG USERNAME=node
RUN useradd -m -s /bin/zsh $USERNAME || true

# Persist bash history
RUN mkdir -p /commandhistory && \
  touch /commandhistory/.bash_history && \
  chown -R $USERNAME /commandhistory

# Create workspace and config directories
RUN mkdir -p /workspace /home/$USERNAME/.claude /home/$USERNAME/go && \
  chown -R $USERNAME:$USERNAME /workspace /home/$USERNAME/.claude /home/$USERNAME/go

WORKDIR /workspace

# Set up zsh with powerline10k theme
ARG ZSH_IN_DOCKER_VERSION=1.2.0
USER $USERNAME
RUN sh -c "$(wget -O- https://github.com/deluan/zsh-in-docker/releases/download/v${ZSH_IN_DOCKER_VERSION}/zsh-in-docker.sh)" -- \
  -p git \
  -p fzf \
  -a "source /usr/share/doc/fzf/examples/key-bindings.zsh" \
  -a "source /usr/share/doc/fzf/examples/completion.zsh" \
  -a "export PROMPT_COMMAND='history -a' && export HISTFILE=/commandhistory/.bash_history" \
  -x

# Pre-populate known_hosts (avoids interactive prompt on first git pull)
USER root
RUN mkdir -p /home/$USERNAME/.ssh && \
  ssh-keyscan bitbucket.org github.com >> /home/$USERNAME/.ssh/known_hosts 2>/dev/null && \
  chown -R $USERNAME:$USERNAME /home/$USERNAME/.ssh && \
  chmod 700 /home/$USERNAME/.ssh && \
  chmod 644 /home/$USERNAME/.ssh/known_hosts

# Copy and configure firewall script
COPY init-firewall.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/init-firewall.sh && \
  echo "$USERNAME ALL=(root) NOPASSWD: /usr/local/bin/init-firewall.sh, /usr/bin/chmod 666 /ssh-agent" > /etc/sudoers.d/$USERNAME-firewall && \
  chmod 0440 /etc/sudoers.d/$USERNAME-firewall

# Environment
ENV DEVCONTAINER=true
ENV SHELL=/bin/zsh
ENV EDITOR=nano
ENV VISUAL=nano
ENV GOPATH=/home/node/go
ENV PATH="${GOPATH}/bin:${PATH}"

USER $USERNAME
