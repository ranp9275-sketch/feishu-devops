import React, { useEffect, useState } from 'react';
import { Card, Col, Row, Statistic, Typography, Spin } from 'antd';
import { CheckCircleOutlined, ApiOutlined } from '@ant-design/icons';
import { getVersion, getLatestJson } from '../services/api';

const { Title } = Typography;

const Dashboard: React.FC = () => {
  const [version, setVersion] = useState<string>('-');
  const [loading, setLoading] = useState(true);
  const [latestData, setLatestData] = useState<any>(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const vRes = await getVersion();
        if (vRes.code === 0) {
          setVersion(vRes.data.version);
        }
        
        const lRes = await getLatestJson();
        if (lRes.code === 0) {
          setLatestData(lRes.data);
        }
      } catch (error) {
        console.error('Failed to fetch dashboard data', error);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, []);

  return (
    <div style={{ padding: '24px' }}>
      <Title level={2}>Dashboard</Title>
      
      <Spin spinning={loading}>
        <Row gutter={16}>
          <Col span={8}>
            <Card>
              <Statistic
                title="System Version"
                value={version}
                prefix={<ApiOutlined />}
              />
            </Card>
          </Col>
          <Col span={8}>
            <Card>
              <Statistic
                title="System Status"
                value="Healthy"
                prefix={<CheckCircleOutlined />}
                valueStyle={{ color: '#3f8600' }}
              />
            </Card>
          </Col>
        </Row>

        <div style={{ marginTop: '24px' }}>
          <Card title="Latest OA Data Snippet">
            <pre style={{ maxHeight: '300px', overflow: 'auto' }}>
              {latestData ? JSON.stringify(latestData, null, 2) : 'No data available'}
            </pre>
          </Card>
        </div>
      </Spin>
    </div>
  );
};

export default Dashboard;
