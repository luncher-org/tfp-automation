output "rke2_server1_public_ip" {
  value = aws_instance.rke2_server1.ipv6_addresses[0]
}

output "rke2_server2_public_ip" {
  value = aws_instance.rke2_server2.ipv6_addresses[0]
}

output "rke2_server3_public_ip" {
  value = aws_instance.rke2_server3.ipv6_addresses[0]
}