# typed: false
# frozen_string_literal: true

# CLI for Scalr remote state & operations backend
class ScalrCli < Formula
  desc "CLI for Scalr remote state & operations backend"
  homepage "https://scalr.com"
  url "https://github.com/Scalr/scalr-cli/archive/refs/tags/v0.17.2.tar.gz"
  sha256 "0bbc9661612a2a8822c2da8225a39567e240f2ddf8d99903456c234ed1b37477"
  license "Apache-2.0"
  head "https://github.com/Scalr/scalr-cli.git", branch: "main"

  depends_on "go" => :build

  def install
    # Get build information (with fallbacks for build environment)
    git_commit = begin
      Utils.safe_popen_read("git", "rev-parse", "HEAD").chomp
    rescue
      "unknown"
    end
    build_date = Time.now.utc.strftime("%Y-%m-%dT%H:%M:%SZ")

    # Build with dynamic version information
    system "go", "build",
           "-ldflags", "-s -w -X main.versionCLI=#{version} -X main.buildDate=#{build_date}",
           "-o", bin/"scalr", "."
  end

  test do
    # Test that the binary runs and shows version
    output = shell_output("#{bin}/scalr -version")

    # Check if it contains version information (handles both old and new format)
    assert_match(/scalr-cli version/, output)
  end
end