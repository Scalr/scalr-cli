class ScalrCli < Formula
  desc "CLI for Scalr remote state & operations backend"
  homepage "https://github.com/Scalr/scalr-cli"
  url "https://github.com/Scalr/scalr-cli/archive/refs/tags/v0.17.1.tar.gz"
  sha256 "c243e739127d47e1eb8093c2ba91bc76f5baa421bd6a7456a0a2495d621afb76"
  license "Apache-2.0"

  depends_on "go" => :build

  def install
    # Get build information (with fallbacks for build environment)
    git_commit = Utils.safe_popen_read("git", "rev-parse", "HEAD").chomp rescue "unknown"
    build_date = Time.now.utc.strftime("%Y-%m-%dT%H:%M:%SZ")
    
    # Build with dynamic version information
    system "go", "build", 
           "-ldflags", "-s -w -X main.versionCLI=#{version} -X main.gitCommit=#{git_commit} -X main.buildDate=#{build_date}",
           "-o", bin/"scalr", "."
  end

  test do
    # Test that the binary runs and shows version
    output = shell_output("#{bin}/scalr -version")
    
    # Check if it contains version information (handles both old and new format)
    assert_match(/#{version}/, output)
  end
end