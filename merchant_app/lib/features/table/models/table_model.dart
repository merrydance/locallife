class TableTagInfo {
  final int id;
  final String name;
  final String type;

  TableTagInfo({required this.id, required this.name, required this.type});

  factory TableTagInfo.fromJson(Map<String, dynamic> json) {
    return TableTagInfo(
      id: json['id'] as int,
      name: json['name'] as String,
      type: json['type'] as String? ?? '',
    );
  }
}

class ReservationInfo {
  final int id;
  final String contactName;
  final String contactPhone;
  final int guestCount;
  final String reservationTime;
  final String? notes;

  ReservationInfo({
    required this.id,
    required this.contactName,
    required this.contactPhone,
    required this.guestCount,
    required this.reservationTime,
    this.notes,
  });

  factory ReservationInfo.fromJson(Map<String, dynamic> json) {
    return ReservationInfo(
      id: json['id'] as int,
      contactName: json['contact_name'] as String? ?? '',
      contactPhone: json['contact_phone'] as String? ?? '',
      guestCount: json['guest_count'] as int? ?? 0,
      reservationTime: json['reservation_time'] as String? ?? '',
      notes: json['notes'] as String?,
    );
  }
}

enum TableStatus {
  available('available', '空闲中'),
  occupied('occupied', '就餐中'),
  disabled('disabled', '已停用');

  final String value;
  final String label;
  const TableStatus(this.value, this.label);

  static TableStatus fromString(String? val) {
    return TableStatus.values.firstWhere(
      (e) => e.value == val,
      orElse: () => TableStatus.available,
    );
  }
}

enum TableType {
  table('table', '大厅散座'),
  room('room', '包间');

  final String value;
  final String label;
  const TableType(this.value, this.label);

  static TableType fromString(String? val) {
    return TableType.values.firstWhere(
      (e) => e.value == val,
      orElse: () => TableType.table,
    );
  }
}

class TableModel {
  final int id;
  final int merchantId;
  final String tableNo;
  final TableType tableType;
  final int capacity;
  final String? description;
  final int? minimumSpend;
  final String? qrCodeUrl;
  final TableStatus status;
  final int? currentReservationId;
  final ReservationInfo? currentReservation;
  final String? imageUrl;
  final List<TableTagInfo> tags;

  TableModel({
    required this.id,
    required this.merchantId,
    required this.tableNo,
    required this.tableType,
    required this.capacity,
    this.description,
    this.minimumSpend,
    this.qrCodeUrl,
    required this.status,
    this.currentReservationId,
    this.currentReservation,
    this.imageUrl,
    this.tags = const [],
  });

  factory TableModel.fromJson(Map<String, dynamic> json) {
    return TableModel(
      id: json['id'] as int,
      merchantId: json['merchant_id'] as int,
      tableNo: json['table_no'] as String,
      tableType: TableType.fromString(json['table_type'] as String?),
      capacity: json['capacity'] as int? ?? 4,
      description: json['description'] as String?,
      minimumSpend: json['minimum_spend'] as int?,
      qrCodeUrl: json['qr_code_url'] as String?,
      status: TableStatus.fromString(json['status'] as String?),
      currentReservationId: json['current_reservation_id'] as int?,
      currentReservation: json['current_reservation'] != null
          ? ReservationInfo.fromJson(json['current_reservation'])
          : null,
      imageUrl: json['image_url'] as String?,
      tags:
          (json['tags'] as List<dynamic>?)
              ?.map((e) => TableTagInfo.fromJson(e))
              .toList() ??
          [],
    );
  }

  TableModel copyWith({
    int? id,
    int? merchantId,
    String? tableNo,
    TableType? tableType,
    int? capacity,
    String? description,
    int? minimumSpend,
    String? qrCodeUrl,
    TableStatus? status,
    int? currentReservationId,
    ReservationInfo? currentReservation,
    String? imageUrl,
    List<TableTagInfo>? tags,
  }) {
    return TableModel(
      id: id ?? this.id,
      merchantId: merchantId ?? this.merchantId,
      tableNo: tableNo ?? this.tableNo,
      tableType: tableType ?? this.tableType,
      capacity: capacity ?? this.capacity,
      description: description ?? this.description,
      minimumSpend: minimumSpend ?? this.minimumSpend,
      qrCodeUrl: qrCodeUrl ?? this.qrCodeUrl,
      status: status ?? this.status,
      currentReservationId: currentReservationId ?? this.currentReservationId,
      currentReservation: currentReservation ?? this.currentReservation,
      imageUrl: imageUrl ?? this.imageUrl,
      tags: tags ?? this.tags,
    );
  }
}
