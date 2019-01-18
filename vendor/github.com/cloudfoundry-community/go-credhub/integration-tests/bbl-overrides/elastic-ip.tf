resource "google_compute_address" "ip_address" {
  name = "go-credhub-external-ip"
}

output "go-credhub-external-ip" {
    value = "${google_compute_address.ip_address.address}"
}