variable "name" { type = string }
variable "node" { type = string }
variable "group" { type = string }
variable "user_data_file_id" { type = string }

#Initialize cpu variables
variable "cores" { type = number }
variable "sockets" { type = number }
variable "type" { type = string }

#Initialize memory
variable "ram" { type = number }

#Initialize Disk
variable "disk_size" { type = number }
variable "datastore_id" { type = string }
variable "file_id" { type = string }
variable "interface" { type = string }
variable "iothread" { type = string }

#Initialize Network_device 
variable "bridge" { type = string }

variable "vlan_id" {
  description = "802.1Q tag for the VM's network_device. 0 = untagged (the vlan_id is omitted from the block); 1-4094 = tagged. Root variables.tf validates the range."
  type        = number
}

variable "model" { type = string }

#initialize ip
variable "ip_address" { type = string }
variable "dns" { type = list(string) }

variable "gateway" {
  type    = string
  default = null
}


