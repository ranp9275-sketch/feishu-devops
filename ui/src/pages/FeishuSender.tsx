import React, { useState } from 'react';
import { Form, Input, Button, Select, Card, Typography, message, Radio } from 'antd';
import { SendOutlined } from '@ant-design/icons';
import { sendCard } from '../services/api';

const { Title } = Typography;
const { TextArea } = Input;
const { Option } = Select;

const FeishuSender: React.FC = () => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const onFinish = async (values: any) => {
    setLoading(true);
    try {
      // Validate JSON content
      const content = JSON.parse(values.content);
      
      // Construct payload according to backend expectation
      // Backend expects: { receive_id, receive_id_type, msg_type, content: JSON_STRING }
      const payload = {
        receive_id: values.receive_id,
        receive_id_type: values.receive_id_type,
        msg_type: values.msg_type,
        content: JSON.stringify(content) // Double stringify as per backend handler requirement
      };

      await sendCard(payload);
      message.success('Message sent successfully!');
    } catch (error) {
      if (error instanceof SyntaxError) {
        message.error('Invalid JSON content');
      } else {
        message.error('Failed to send message');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ padding: '24px', maxWidth: '800px', margin: '0 auto' }}>
      <Title level={2}>Feishu Message Sender</Title>
      <Card>
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          initialValues={{
            receive_id_type: 'open_id',
            msg_type: 'interactive',
            content: '{\n  "config": {\n    "wide_screen_mode": true\n  },\n  "elements": [\n    {\n      "tag": "div",\n      "text": {\n        "content": "This is a test message",\n        "tag": "lark_md"\n      }\n    }\n  ],\n  "header": {\n    "template": "blue",\n    "title": {\n      "content": "Test Card",\n      "tag": "plain_text"\n    }\n  }\n}'
          }}
        >
          <Form.Item
            name="receive_id"
            label="Receive ID"
            rules={[{ required: true, message: 'Please input receive ID!' }]}
          >
            <Input placeholder="e.g. ou_xxx or chat_xxx" />
          </Form.Item>

          <Form.Item
            name="receive_id_type"
            label="ID Type"
            rules={[{ required: true }]}
          >
            <Radio.Group>
              <Radio value="open_id">Open ID</Radio>
              <Radio value="user_id">User ID</Radio>
              <Radio value="chat_id">Chat ID</Radio>
              <Radio value="email">Email</Radio>
            </Radio.Group>
          </Form.Item>

          <Form.Item
            name="msg_type"
            label="Message Type"
            rules={[{ required: true }]}
          >
            <Select>
              <Option value="interactive">Interactive (Card)</Option>
              <Option value="text">Text</Option>
            </Select>
          </Form.Item>

          <Form.Item
            name="content"
            label="Content (JSON)"
            rules={[{ required: true, message: 'Please input JSON content!' }]}
            tooltip="For interactive cards, provide the card JSON structure."
          >
            <TextArea rows={12} style={{ fontFamily: 'monospace' }} />
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" icon={<SendOutlined />} loading={loading} block>
              Send Message
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default FeishuSender;
