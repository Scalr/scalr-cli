class ScalrCli < Formula
  desc "Scalr CLI for managing Scalr resources"
  homepage "https://github.com/Scalr/scalr-cli"
  url "https://github.com/Scalr/scalr-cli/releases/download/v0.17.1/scalr-cli_0.17.1_darwin_amd64.zip"
  sha256 "" # You'll need to fill this with the actual SHA256 from the release
  license "Apache-2.0"

  depends_on "go" => :build

  def install
    bin.install "scalr"
  end

  test do
    system "#{bin}/scalr", "-version"
  end
end 