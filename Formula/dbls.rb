class Dhumal < Formula
  desc "TUI/CLI tool for interacting with Postgres"
  homepage "https://github.com/hrutik5321/dbls"
  url "https://github.com/hrutik5321/dbls/archive/refs/tags/v1.0.0.tar.gz"
  sha256 "cb4b4b1d09b13e32c156a3bf7cd43d18abcf019b96684113961654c779074216"
  license "MIT" # or your actual license

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(output: bin/"dbls"), "."
  end

  test do
    system "#{bin}/dbls", "--help"
  end
end
